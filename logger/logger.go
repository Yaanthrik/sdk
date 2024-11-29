package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Log levels
const (
	INFO  = "INFO"
	DEBUG = "DEBUG"
	WARN  = "WARN"
	ERROR = "ERROR"
	FATAL = "FATAL"
)

// Config defines the logger configuration
type Config struct {
	Mode        string // "console" or "file"
	FilePath    string // Path for log file (if Mode is "file")
	JSONFormat  bool   // If true, use JSON log format
	LogLevel    string // Minimum log level to log
	EnableAsync bool   // Enable asynchronous logging
}

var globalConfig = Config{
	Mode:        "console",
	JSONFormat:  false,
	LogLevel:    DEBUG,
	EnableAsync: false,
}

var logChannel chan LogEntry
var wg sync.WaitGroup

// LogEntry defines the structure of a log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	File      string                 `json:"file"`
	Line      int                    `json:"line"`
	Function  string                 `json:"function"`
	Message   string                 `json:"message"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Error     string                 `json:"error,omitempty"`
	ProcessID int                    `json:"pid"`
}

// Initialize sets the global logger configuration
func Initialize(config Config) {
	globalConfig = config
	if globalConfig.EnableAsync {
		logChannel = make(chan LogEntry, 100)
		go processLogQueue()
	}
}

// processLogQueue processes log entries asynchronously
func processLogQueue() {
	for entry := range logChannel {
		writeLog(entry)
		wg.Done()
	}
}

// getCallerInfo retrieves the file, line number, and function name of the caller
func getCallerInfo(skip int) (file string, line int, funcName string) {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		file = "unknown"
		line = 0
		funcName = "unknown"
	} else {
		funcName = runtime.FuncForPC(pc).Name()
		file = trimFilePath(file)
	}
	return file, line, funcName
}

// trimFilePath trims the file path to show only the file name
func trimFilePath(fullPath string) string {
	parts := strings.Split(fullPath, "/")
	return parts[len(parts)-1]
}

// isLogLevelEnabled checks if the current log level is enabled
func isLogLevelEnabled(level string) bool {
	levels := map[string]int{
		DEBUG: 1, INFO: 2, WARN: 3, ERROR: 4, FATAL: 5,
	}
	return levels[level] >= levels[globalConfig.LogLevel]
}

// log constructs and logs the entry based on the global configuration
func log(level, msg string, payload map[string]interface{}, err error) {
	if !isLogLevelEnabled(level) {
		return
	}

	file, line, funcName := getCallerInfo(3) // Adjust skip for logger calls
	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		File:      file,
		Line:      line,
		Function:  funcName,
		Message:   msg,
		Payload:   payload,
		Error:     "",
		ProcessID: os.Getpid(),
	}

	if err != nil {
		entry.Error = err.Error()
	}

	if globalConfig.EnableAsync {
		wg.Add(1)
		logChannel <- entry
	} else {
		writeLog(entry)
	}
}

// writeLog writes the log entry based on the global configuration
func writeLog(entry LogEntry) {
	if globalConfig.Mode == "console" {
		outputConsole(entry)
	} else if globalConfig.Mode == "file" && globalConfig.FilePath != "" {
		outputFile(entry)
	}
}

// outputConsole prints the log entry to the console
func outputConsole(entry LogEntry) {
	if globalConfig.JSONFormat {
		data, _ := json.Marshal(entry)
		fmt.Println(string(data))
	} else {
		fmt.Printf("[%s] %s %s:%d %s - %s\n",
			entry.Timestamp, entry.Level, entry.File, entry.Line, entry.Function, entry.Message)
		if entry.Error != "" {
			fmt.Printf("Error: %s\n", entry.Error)
		}
		if entry.Payload != nil {
			fmt.Printf("Payload: %+v\n", entry.Payload)
		}
	}
}

// outputFile writes the log entry to a file
func outputFile(entry LogEntry) {
	var data string
	if globalConfig.JSONFormat {
		jsonData, _ := json.Marshal(entry)
		data = string(jsonData)
	} else {
		data = fmt.Sprintf("[%s] %s %s:%d %s - %s",
			entry.Timestamp, entry.Level, entry.File, entry.Line, entry.Function, entry.Message)
		if entry.Error != "" {
			data += fmt.Sprintf("\nError: %s", entry.Error)
		}
	}

	f, err := os.OpenFile(globalConfig.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Failed to write log:", err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(data + "\n"); err != nil {
		fmt.Println("Failed to write log data:", err)
	}
}

// CloseLogger waits for all logs to be processed and closes the log channel
func CloseLogger() {
	if globalConfig.EnableAsync {
		wg.Wait()
		close(logChannel)
	}
}

// Public logging functions

// Info logs an informational message
func Info(msg string, payload map[string]interface{}) {
	log(INFO, msg, payload, nil)
}

// Debug logs a debug message
func Debug(msg string, payload map[string]interface{}) {
	log(DEBUG, msg, payload, nil)
}

// Warn logs a warning message
func Warn(msg string, payload map[string]interface{}) {
	log(WARN, msg, payload, nil)
}

// Error logs an error message
func Error(msg string, err error, payload map[string]interface{}) {
	log(ERROR, msg, payload, err)
}

// Fatal logs a fatal error and exits
func Fatal(msg string, err error, payload map[string]interface{}) {
	log(FATAL, msg, payload, err)
	CloseLogger() // Ensure all async logs are processed before exiting
	os.Exit(1)
}

// WrapError wraps an error with file and line number information
func WrapError(err error) error {
	if err == nil {
		return nil
	}
	file, line, _ := getCallerInfo(2)
	return fmt.Errorf("%s:%d: %w", file, line, err)
}
