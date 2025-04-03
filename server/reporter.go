package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type ReporterRegister struct {
	mutex     sync.Mutex
	reporters map[string]ReporterInfo
}

type ReporterInfo struct {
	Name       string `json:"name"`
	Callback   string `json:"callback"`
	LiveReport bool   `json:"live"`
}

func (r *ReporterRegister) AddReporter(reporter ReporterInfo) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.reporters[reporter.Name] = reporter
}

func (r *ReporterRegister) RemoveReporter(name string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.reporters, name)
}

func (r *ReporterRegister) GetReporters() map[string]ReporterInfo {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.reporters
}

var reporters ReporterRegister

func init() {
	reporters = ReporterRegister{
		reporters: make(map[string]ReporterInfo),
	}
}

type ReporterDeleteRequest struct {
	Name string `json:"name"`
}

func registerReporter(w http.ResponseWriter, r *http.Request) {
	reporters.mutex.Lock()
	defer reporters.mutex.Unlock()

	if r.Method == http.MethodPost {
		var req ReporterInfo
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.Callback == "" {
			req.Callback = fmt.Sprintf("http://%s:8080/", strings.Split(r.RemoteAddr, ":")[0])
		}

		reporters.AddReporter(ReporterInfo(req))
		logger.With("name", req.Name, "callback", req.Callback, "live_report", req.LiveReport).Info("New reporter registered.")
		return
	} else if r.Method == http.MethodDelete {
		var req ReporterDeleteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		reporters.RemoveReporter(req.Name)
		logger.With("name", req.Name).Info("Reporter deleted.")
		return
	}

	http.Error(w, "invalid HTTP method", http.StatusBadRequest)
}

func sendSessionReport(session *SessionInfo) {
	reporters.mutex.Lock()
	defer reporters.mutex.Unlock()

	for _, reporter := range reporters.reporters {
		go endReportBy(&reporter, session)
	}
}

func endReportBy(reporter *ReporterInfo, session *SessionInfo) {
	reportURL := fmt.Sprintf("%sreporter/%s/report", reporter.Callback, strings.ToLower(reporter.Name))
	reqJSON, err := json.Marshal(session)
	if err != nil {
		logger.With("reporter", reporter.Name, "error", err, "session", session.UUID.String()).Error("Failed to marshal session report.")
		return
	}

	resp, err := http.Post(reportURL, "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		logger.With("reporter", reporter.Name, "error", err, "session", session.UUID.String()).Error("Failed to send session report.")
		return
	}

	if resp.StatusCode != http.StatusOK {
		logger.With("reporter", reporter.Name, "status", resp.Status, "session", session.UUID.String()).Error("Session report failed. Check Reporter log.")
	}

	resp.Body.Close()
}

type LiveLogMessageData struct {
	UUID    string            `json:"session"`
	Message SessionLogMessage `json:"message"`
}

func sendLiveLogMessage(session *SessionInfo, logMessage SessionLogMessage) {
	reporters.mutex.Lock()
	defer reporters.mutex.Unlock()

	for _, reporter := range reporters.reporters {
		if !reporter.LiveReport {
			continue
		}

		reportURL := fmt.Sprintf("%sreporter/%s/live", reporter.Callback, strings.ToLower(reporter.Name))
		data := LiveLogMessageData{
			UUID:    session.UUID.String(),
			Message: logMessage,
		}
		reqJSON, err := json.Marshal(data)
		if err != nil {
			logger.With("reporter", reporter.Name, "error", err, "session", session.UUID.String()).Error("Failed to marshal live log message.")
			continue
		}

		resp, err := http.Post(reportURL, "application/json", bytes.NewBuffer(reqJSON))
		if err != nil {
			logger.With("reporter", reporter.Name, "error", err, "session", session.UUID.String()).Error("Failed to send live log message.")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			logger.With("reporter", reporter.Name, "status", resp.Status, "session", session.UUID.String()).Error("Live logging to reporter failed. Check Reporter log.")
		}

		resp.Body.Close()
	}
}

type reporterRegisterRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Callback string `json:"callback"`
	Secret   string `json:"secret"`
	Live     bool   `json:"live"`
}

type reporterDeleteRequest struct {
	Name string `json:"name"`
}

func setupPreconfiguredReporter(name string) {
	reporters.mutex.Lock()
	defer reporters.mutex.Unlock()

	if !viper.IsSet(fmt.Sprintf("reporter.%s.callback", name)) {
		logger.With("reporter", name).Error("Preconfigured reporter is missing callback.")
		return
	}

	callback := viper.GetString(fmt.Sprintf("reporter.%s.callback", name))
	secret := viper.GetString(fmt.Sprintf("reporter.%s.secret", name))
	if !viper.IsSet(fmt.Sprintf("reporter.%s.secret", name)) {
		secret = ""
	}

	if !strings.HasSuffix(callback, "/") {
		callback += "/"
	}

	reporterURL := fmt.Sprintf("%sreporter/%s/serverConnect", callback, name)
	req := ReporterInfo{
		Callback: fmt.Sprintf("http://%s:%d/", viper.GetString("hostname"), viper.GetInt("port")),
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		logger.With("reporter", name, "error", err).Error("Failed to marshal server side registration request.")
		return
	}

	resp, err := http.Post(reporterURL, "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		logger.With("reporter", name, "error", err).Error("Failed to attach server to reporter.")
		return
	}

	if resp.StatusCode != http.StatusOK {
		logger.With("reporter", name, "statusCode", resp.StatusCode).Error("Failed to attach server to reporter. Check reporter logs.")
		return
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.With("reporter", name, "error", err).Error("Failed reading reporter response on server side registration.")
		return
	}

	var result reporterRegisterRequest
	if err := json.Unmarshal(body, &result); err != nil {
		logger.With("reporter", name, "error", err).Error("Failed parsing reporter response on server side registration.")
		return
	}

	if result.Secret != secret {
		logger.With("reporter", name).Error("Server side reporter registration aborted. Invalid secret from reporter.")
		return
	}

	reporters.reporters[name] = ReporterInfo{
		Name:       result.Name,
		Callback:   result.Callback,
		LiveReport: result.Live,
	}
	logger.With("reporter", name).Info("Server side reporter registered.")
}
