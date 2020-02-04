// Package name declaration
package logger

// Import packages
import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

var (
	// Map for the various codes of colors
	colors map[LogLevel]string

	// Map from format's placeholders to printf verbs
	phfs map[string]string

	// Contains color strings for stdout
	logNo uint64

	// Default format of log message
	defFmt = "#%[1]d %[2]s %[4]s:%[5]d ▶ %.3[6]s %[7]s"

	// Default format of time
	defTimeFmt = "2006-01-02 15:04:05"
)

// LogLevel type
type LogLevel int

// Color numbers for stdout
const (
	Black = (iota + 30)
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
)

// Log Level
const (
	CriticalLevel LogLevel = iota + 1
	ErrorLevel
	WarningLevel
	NoticeLevel
	InfoLevel
	DebugLevel
)

// Worker class, Worker is a log object used to log messages and Color specifies
// if colored output is to be produced
type Worker struct {
	Minion     *log.Logger
	Color      int
	format     string
	timeFormat string
	level      LogLevel
}

// Record class, Contains all the info on what has to logged, time is the current time, Module is the specific module
// For which we are logging, level is the state, importance and type of message logged,
// Message contains the string to be logged, format is the format of string to be passed to sprintf
type Record struct {
	Id       uint64
	Time     string
	Module   string
	Level    LogLevel
	Line     int
	Filename string
	Message  string
	//format   string
}

// Logger class that is an interface to user to log messages, Module is the module for which we are testing
// worker is variable of Worker class that is used in bottom layers to log the message
type Logger struct {
	Module string
	worker *Worker
}

// init pkg
func init() {
	initColors()
	initFormatPlaceholders()
}

// Returns a proper string to be outputted for a particular info
func (info *Record) Output(format string) string {
	msg := fmt.Sprintf(format,
		info.Id,               // %[1] // %{id}
		info.Time,             // %[2] // %{time[:fmt]}
		info.Module,           // %[3] // %{module}
		info.Filename,         // %[4] // %{filename}
		info.Line,             // %[5] // %{line}
		info.logLevelString(), // %[6] // %{level}
		info.Message,          // %[7] // %{message}
	)
	// Ignore printf errors if len(args) > len(verbs)
	if i := strings.LastIndex(msg, "%!(EXTRA"); i != -1 {
		return msg[:i]
	}
	return msg
}

// Analyze and represent format string as printf format string and time format
func parseFormat(format string) (msgfmt, timefmt string) {
	if len(format) < 10 /* (len of "%{message} */ {
		return defFmt, defTimeFmt
	}
	timefmt = defTimeFmt
	idx := strings.IndexRune(format, '%')
	for idx != -1 {
		msgfmt += format[:idx]
		format = format[idx:]
		if len(format) > 2 {
			if format[1] == '{' {
				// end of curr verb pos
				if jdx := strings.IndexRune(format, '}'); jdx != -1 {
					// next verb pos
					idx = strings.Index(format[1:], "%{")
					// incorrect verb found ("...%{wefwef ...") but after
					// this, new verb (maybe) exists ("...%{inv %{verb}...")
					if idx != -1 && idx < jdx {
						msgfmt += "%%"
						format = format[1:]
						continue
					}
					// get verb and arg
					verb, arg := ph2verb(format[:jdx+1])
					msgfmt += verb
					// check if verb is time
					// here you can handle args for other verbs
					if verb == `%[2]s` && arg != "" /* %{time} */ {
						timefmt = arg
					}
					format = format[jdx+1:]
				} else {
					format = format[1:]
				}
			} else {
				msgfmt += "%%"
				format = format[1:]
			}
		}
		idx = strings.IndexRune(format, '%')
	}
	msgfmt += format
	return
}

// translate format placeholder to printf verb and some argument of placeholder
// (now used only as time format)
func ph2verb(ph string) (verb string, arg string) {
	n := len(ph)
	if n < 4 {
		return ``, ``
	}
	if ph[0] != '%' || ph[1] != '{' || ph[n-1] != '}' {
		return ``, ``
	}
	idx := strings.IndexRune(ph, ':')
	if idx == -1 {
		return phfs[ph], ``
	}
	verb = phfs[ph[:idx]+"}"]
	arg = ph[idx+1 : n-1]
	return
}

// Returns an instance of worker class, prefix is the string attached to every log,
// flag determine the log params, color parameters verifies whether we need colored outputs or not
func NewWorker(prefix string, flag int, color int, out io.Writer) *Worker {
	return &Worker{Minion: log.New(out, prefix, flag), Color: color, format: defFmt, timeFormat: defTimeFmt}
}

func SetDefaultFormat(format string) {
	defFmt, defTimeFmt = parseFormat(format)
}

func (w *Worker) SetFormat(format string) {
	w.format, w.timeFormat = parseFormat(format)
}

func (w *Worker) SetLogLevel(level LogLevel) {
	w.level = level
}

// Function of Worker class to log a string based on level
func (w *Worker) Log(level LogLevel, calldepth int, record *Record) error {

	if w.level < level {
		return nil
	}

	if w.Color != 0 {
		buf := &bytes.Buffer{}
		buf.Write([]byte(colors[level]))
		buf.Write([]byte(record.Output(w.format)))
		buf.Write([]byte("\033[0m"))
		return w.Minion.Output(calldepth+1, buf.String())
	} else {
		return w.Minion.Output(calldepth+1, record.Output(w.format))
	}
}

// Returns a proper string to output for colored logging
func colorString(color int) string {
	return fmt.Sprintf("\033[%dm", int(color))
}

// Initializes the map of colors
func initColors() {
	colors = map[LogLevel]string{
		CriticalLevel: colorString(Magenta),
		ErrorLevel:    colorString(Red),
		WarningLevel:  colorString(Yellow),
		NoticeLevel:   colorString(Green),
		DebugLevel:    colorString(Cyan),
		InfoLevel:     colorString(White),
	}
}

// Initializes the map of placeholders
func initFormatPlaceholders() {
	phfs = map[string]string{
		"%{id}":       "%[1]d",
		"%{time}":     "%[2]s",
		"%{module}":   "%[3]s",
		"%{filename}": "%[4]s",
		"%{file}":     "%[4]s",
		"%{line}":     "%[5]d",
		"%{level}":    "%[6]s",
		"%{lvl}":      "%.3[6]s",
		"%{message}":  "%[7]s",
	}
}

// Returns a new instance of logger class, module is the specific module for which we are logging
// , color defines whether the output is to be colored or not, out is instance of type io.Writer defaults
// to os.Stderr
func New(args ...interface{}) *Logger {
	//initColors()

	var module string = "DEFAULT"
	var color int = 1
	var out io.Writer = os.Stderr
	var level LogLevel = InfoLevel

	for _, arg := range args {
		switch t := arg.(type) {
		case string:
			module = t
		case int:
			color = t
		case io.Writer:
			out = t
		case LogLevel:
			level = t
		default:
			panic("logger: Unknown argument")
		}
	}
	newWorker := NewWorker("", 0, color, out)
	newWorker.SetLogLevel(level)
	return &Logger{Module: module, worker: newWorker}
}

func anyToMessage(format string, a ...interface{}) string {
	if format == "" {
		format = strings.TrimRight(strings.Repeat("%v ", len(a)), " ")
	}
	return fmt.Sprintf(format, a...)
}

func (l *Logger) SetFormat(format string) {
	l.worker.SetFormat(format)
}

func (l *Logger) SetLogLevel(level LogLevel) {
	l.worker.level = level
}

func (l *Logger) SetLogColor(color int) {
	l.worker.Color = color
}

func (l *Logger) logInternal(callDepth int, level LogLevel, message string) {
	//var formatString string = "#%d %s [%s] %s:%d ▶ %.3s %s"
	_, filename, line, _ := runtime.Caller(callDepth)
	filename = path.Base(filename)
	info := &Record{
		Id:       atomic.AddUint64(&logNo, 1),
		Time:     time.Now().Format(l.worker.timeFormat),
		Module:   l.Module,
		Level:    level,
		Message:  message,
		Filename: filename,
		Line:     line,
		//format:   formatString,
	}
	_ = l.worker.Log(level, callDepth, info)
}

func (l *Logger) LogF(callDepth int, level LogLevel, format string, a ...interface{}) {
	l.logInternal(callDepth, level, anyToMessage(format, a...))
}

func (l *Logger) Log(callDepth int, level LogLevel, a ...interface{}) {
	l.logInternal(callDepth, level, anyToMessage("", a...))
}

// Fatal is just like func l.Critical logger except that it is followed by exit to program
func (l *Logger) Fatal(a ...interface{}) {
	l.logInternal(2, CriticalLevel, anyToMessage("", a...))
	os.Exit(1)
}

// FatalF is just like func l.CriticalF logger except that it is followed by exit to program
func (l *Logger) FatalF(format string, a ...interface{}) {
	l.logInternal(2, CriticalLevel, anyToMessage(format, a...))
	os.Exit(1)
}

// FatalF is just like func l.CriticalF logger except that it is followed by exit to program
func (l *Logger) Fatalf(format string, a ...interface{}) {
	l.logInternal(2, CriticalLevel, anyToMessage(format, a...))
	os.Exit(1)
}

// Panic is just like func l.Critical except that it is followed by a call to panic
func (l *Logger) Panic(a ...interface{}) {
	message := anyToMessage("", a...)
	l.logInternal(2, CriticalLevel, message)
	panic(message)
}

// PanicF is just like func l.CriticalF except that it is followed by a call to panic
func (l *Logger) PanicF(format string, a ...interface{}) {
	message := anyToMessage(format, a...)
	l.logInternal(2, CriticalLevel, message)
	panic(message)
}

// PanicF is just like func l.CriticalF except that it is followed by a call to panic
func (l *Logger) Panicf(format string, a ...interface{}) {
	message := anyToMessage(format, a...)
	l.logInternal(2, CriticalLevel, message)
	panic(message)
}

// Critical logs a message at a Critical Level
func (l *Logger) Critical(a ...interface{}) {
	l.logInternal(2, CriticalLevel, anyToMessage("", a...))
}

// CriticalF logs a message at Critical level using the same syntax and options as fmt.Printf
func (l *Logger) CriticalF(format string, a ...interface{}) {
	l.logInternal(2, CriticalLevel, anyToMessage(format, a...))
}

// CriticalF logs a message at Critical level using the same syntax and options as fmt.Printf
func (l *Logger) Criticalf(format string, a ...interface{}) {
	l.logInternal(2, CriticalLevel, anyToMessage(format, a...))
}

// Error logs a message at Error level
func (l *Logger) Error(a ...interface{}) {
	l.logInternal(2, ErrorLevel, anyToMessage("", a...))
}

// ErrorF logs a message at Error level using the same syntax and options as fmt.Printf
func (l *Logger) ErrorF(format string, a ...interface{}) {
	l.logInternal(2, ErrorLevel, anyToMessage(format, a...))
}

// ErrorF logs a message at Error level using the same syntax and options as fmt.Printf
func (l *Logger) Errorf(format string, a ...interface{}) {
	l.logInternal(2, ErrorLevel, anyToMessage(format, a...))
}

// Warning logs a message at Warning level
func (l *Logger) Warning(a ...interface{}) {
	l.logInternal(2, WarningLevel, anyToMessage("", a...))
}

// WarningF logs a message at Warning level using the same syntax and options as fmt.Printf
func (l *Logger) WarningF(format string, a ...interface{}) {
	l.logInternal(2, WarningLevel, anyToMessage(format, a...))
}

// WarningF logs a message at Warning level using the same syntax and options as fmt.Printf
func (l *Logger) Warningf(format string, a ...interface{}) {
	l.logInternal(2, WarningLevel, anyToMessage(format, a...))
}

// Notice logs a message at Notice level
func (l *Logger) Notice(a ...interface{}) {
	l.logInternal(2, NoticeLevel, anyToMessage("", a...))
}

// NoticeF logs a message at Notice level using the same syntax and options as fmt.Printf
func (l *Logger) NoticeF(format string, a ...interface{}) {
	l.logInternal(2, NoticeLevel, anyToMessage(format, a...))
}

// NoticeF logs a message at Notice level using the same syntax and options as fmt.Printf
func (l *Logger) Noticef(format string, a ...interface{}) {
	l.logInternal(2, NoticeLevel, anyToMessage(format, a...))
}

// Info logs a message at Info level
func (l *Logger) Info(a ...interface{}) {
	l.logInternal(2, InfoLevel, anyToMessage("", a...))
}

// InfoF logs a message at Info level using the same syntax and options as fmt.Printf
func (l *Logger) InfoF(format string, a ...interface{}) {
	l.logInternal(2, InfoLevel, anyToMessage(format, a...))
}

// InfoF logs a message at Info level using the same syntax and options as fmt.Printf
func (l *Logger) Infof(format string, a ...interface{}) {
	l.logInternal(2, InfoLevel, anyToMessage(format, a...))
}

// Debug logs a message at Debug level
func (l *Logger) Debug(a ...interface{}) {
	l.logInternal(2, DebugLevel, anyToMessage("", a...))
}

// DebugF logs a message at Debug level using the same syntax and options as fmt.Printf
func (l *Logger) DebugF(format string, a ...interface{}) {
	l.logInternal(2, DebugLevel, anyToMessage(format, a...))
}

// DebugF logs a message at Debug level using the same syntax and options as fmt.Printf
func (l *Logger) Debugf(format string, a ...interface{}) {
	l.logInternal(2, DebugLevel, anyToMessage(format, a...))
}

// Prints this goroutine's execution stack as an error with an optional message at the begining
func (l *Logger) StackAsError(a ...interface{}) {
	message := anyToMessage("", a...)
	if message == "" {
		message = "Stack info"
	}
	message += "\n"
	l.logInternal(2, ErrorLevel, message+Stack())
}

// Prints this goroutine's execution stack as critical with an optional message at the begining
func (l *Logger) StackAsCritical(a ...interface{}) {
	message := anyToMessage("", a...)
	if message == "" {
		message = "Stack info"
	}
	message += "\n"
	l.logInternal(2, CriticalLevel, message+Stack())
}

// Returns a string with the execution stack for this goroutine
func Stack() string {
	buf := make([]byte, 1000000)
	runtime.Stack(buf, false)
	return string(buf)
}

// Returns the loglevel as string
func (info *Record) logLevelString() string {
	logLevels := [...]string{
		"CRITICAL",
		"ERROR",
		"WARNING",
		"NOTICE",
		"INFO",
		"DEBUG",
	}
	return logLevels[info.Level-1]
}

var defaultLogger = New()

func SetFormat(format string) {
	defaultLogger.SetFormat(format)
}

func SetLogLevel(level LogLevel) {
	defaultLogger.SetLogLevel(level)
}

func SetLogColor(color int) {
	defaultLogger.SetLogColor(color)
}

// Fatal is just like func defaultLogger.Critical logger except that it is followed by exit to program
func Fatal(a ...interface{}) {
	defaultLogger.logInternal(2, CriticalLevel, anyToMessage("", a...))
	os.Exit(1)
}

// FatalF is just like func defaultLogger.CriticalF logger except that it is followed by exit to program
func FatalF(format string, a ...interface{}) {
	defaultLogger.logInternal(2, CriticalLevel, anyToMessage(format, a...))
	os.Exit(1)
}

// FatalF is just like func defaultLogger.CriticalF logger except that it is followed by exit to program
func Fatalf(format string, a ...interface{}) {
	defaultLogger.logInternal(2, CriticalLevel, anyToMessage(format, a...))
	os.Exit(1)
}

// Panic is just like func defaultLogger.Critical except that it is followed by a call to panic
func Panic(a ...interface{}) {
	message := anyToMessage("", a...)
	defaultLogger.logInternal(2, CriticalLevel, message)
	panic(message)
}

// PanicF is just like func defaultLogger.CriticalF except that it is followed by a call to panic
func PanicF(format string, a ...interface{}) {
	message := anyToMessage(format, a...)
	defaultLogger.logInternal(2, CriticalLevel, message)
	panic(message)
}

// Critical logs a message at a Critical Level
func Critical(a ...interface{}) {
	defaultLogger.logInternal(2, CriticalLevel, anyToMessage("", a...))
}

// CriticalF logs a message at Critical level using the same syntax and options as fmt.Printf
func CriticalF(format string, a ...interface{}) {
	defaultLogger.logInternal(2, CriticalLevel, anyToMessage(format, a...))
}

// Error logs a message at Error level
func Error(a ...interface{}) {
	defaultLogger.logInternal(2, ErrorLevel, anyToMessage("", a...))
}

// ErrorF logs a message at Error level using the same syntax and options as fmt.Printf
func ErrorF(format string, a ...interface{}) {
	defaultLogger.logInternal(2, ErrorLevel, anyToMessage(format, a...))
}

// Warning logs a message at Warning level
func Warning(a ...interface{}) {
	defaultLogger.logInternal(2, WarningLevel, anyToMessage("", a...))
}

// WarningF logs a message at Warning level using the same syntax and options as fmt.Printf
func WarningF(format string, a ...interface{}) {
	defaultLogger.logInternal(2, WarningLevel, anyToMessage(format, a...))
}

// Notice logs a message at Notice level
func Notice(a ...interface{}) {
	defaultLogger.logInternal(2, NoticeLevel, anyToMessage("", a...))
}

// NoticeF logs a message at Notice level using the same syntax and options as fmt.Printf
func NoticeF(format string, a ...interface{}) {
	defaultLogger.logInternal(2, NoticeLevel, anyToMessage(format, a...))
}

// Info logs a message at Info level
func Info(a ...interface{}) {
	defaultLogger.logInternal(2, InfoLevel, anyToMessage("", a...))
}

// InfoF logs a message at Info level using the same syntax and options as fmt.Printf
func InfoF(format string, a ...interface{}) {
	defaultLogger.logInternal(2, InfoLevel, anyToMessage(format, a...))
}

// Debug logs a message at Debug level
func Debug(a ...interface{}) {
	defaultLogger.logInternal(2, DebugLevel, anyToMessage("", a...))
}

// DebugF logs a message at Debug level using the same syntax and options as fmt.Printf
func DebugF(format string, a ...interface{}) {
	defaultLogger.logInternal(2, DebugLevel, anyToMessage(format, a...))
}

// Prints this goroutine's execution stack as an error with an optional message at the begining
func StackAsError(a ...interface{}) {
	message := anyToMessage("", a...)
	if message == "" {
		message = "Stack info"
	}
	message += "\n"
	defaultLogger.logInternal(2, ErrorLevel, message+Stack())
}

// Prints this goroutine's execution stack as critical with an optional message at the begining
func StackAsCritical(a ...interface{}) {
	message := anyToMessage("", a...)
	if message == "" {
		message = "Stack info"
	}
	message += "\n"
	defaultLogger.logInternal(2, CriticalLevel, message+Stack())
}
