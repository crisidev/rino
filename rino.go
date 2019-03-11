package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/mvdan/xurls"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	APP_NAME    = "rino"
	APP_VERSION = "1.0"
	APP_AUTHOR  = "bigo@crisidev.org"
	APP_SITE    = "https://github.com/crisidev/rino"
)

var (
	run bool
	lg  RinoLogger

	// Flags
	flagDebug          = kingpin.Flag("debug", "enable debug mode").Short('D').Bool()
	flagLink           = kingpin.Flag("link", "link rino with a SSH exposed port using a tag like \"BIGO:4223\"").Short('l').String()
	flagStop           = kingpin.Flag("stop", "send signal 1 to rino using the current link").Short('s').Bool()
	flagServiceDir     = kingpin.Flag("service-dir", "path for service dir for lock and pid files").Short('d').Default(fmt.Sprintf("%s/.rino", os.Getenv("HOME"))).String()
	flagNotifierCmd    = kingpin.Flag("notifier-cmd", "path for terminal-notifier command").Default("/usr/bin/notify-send").Short('N').String()
	flagNotifierSender = kingpin.Flag("notifier-sender", "sender for terminal-notifier command, aka the icon").Default("com.apple.Terminal").Short('S').String()
)

// Simple logging structure
type RinoLogger struct {
	output *log.Logger
	stderr *log.Logger
}

func init() {
	kingpin.Version(APP_VERSION)
	kingpin.Parse()
	lg.SetupLog()
}

// Setup debugging to stderr
func (l RinoLogger) SetupLog() {
	lg.output = log.New(os.Stdout, "", 0)
	lg.output.SetFlags(log.Ldate | log.Ltime)
}

// Logs to stardard output
func (l RinoLogger) Out(msg string) {
	if *flagDebug {
		l.output.Println(msg)
	}
}

// Logs to standard output without carriage return
func (l RinoLogger) OutRaw(msg string) {
	if *flagDebug {
		fmt.Printf(msg)
	}
}

// Logs a fatal error
func (l RinoLogger) Fatal(err error) {
	if err != nil {
		l.output.Printf(fmt.Sprintf("ERROR: %s", err.Error()))
	}
	os.Exit(1)
}

// Logs an erro
func (l RinoLogger) Error(err error) {
	if err != nil {
		l.output.Printf(fmt.Sprintf("ERROR: %s", err.Error()))
	}
}

// Create service dir if not exists
func CreateRinoServiceDir() (err error) {
	if _, err := os.Stat(*flagServiceDir); err != nil {
		lg.Out(fmt.Sprintf("%s not found, creating", *flagServiceDir))
		return os.Mkdir(*flagServiceDir, 0755)
	}
	return
}

// Checks if a PID is running on the system
func CheckIfPidExist(pid int) (retval bool) {
	process, err := os.FindProcess(pid)
	if err != nil {
		lg.Out(fmt.Sprintf("PID %d is not running", pid))
		retval = false
	}
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		if *flagStop {
			Stop(process)
		}
		retval = true
	}
	if err == syscall.ESRCH {
		retval = false
	}
	return
}

// Setup pidfile into service directory. If pidfile is not found, will be created.
// Otherwise the PID will be searched to see if the process is running and if
// it is safe to remove the pidfile.
func SetupPidFile(link string) (err error) {
	pidFile := path.Join(*flagServiceDir, fmt.Sprintf("%s.pid", *flagLink))
	// pidFile does not exists
	lg.Out(fmt.Sprintf("checking PID file %s", pidFile))
	if _, err := os.Stat(pidFile); err != nil {
		return WritePidFile(pidFile)
		// pidFile exists
	} else {
		pidBytes, err := ioutil.ReadFile(pidFile)
		if err != nil {
			return err
		}
		pid, err := strconv.Atoi(string(pidBytes))
		if err != nil {
			return err
		}
		if CheckIfPidExist(pid) {
			return errors.New(fmt.Sprintf("PID %d found into %s still running", pid, pidFile))
		} else {
			return WritePidFile(pidFile)
		}
	}
	return
}

// Write current PID to pidfile
func WritePidFile(pidFile string) (err error) {
	pid := os.Getpid()
	lg.Out(fmt.Sprintf("writing PID %d to %s", pid, pidFile))
	err = ioutil.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
	if err != nil {
		return err
	}
	return err
}

func RinoTCPServer(tag string, port string) (err error) {
	lg.Out(fmt.Sprintf("starting server for tag %s on port %s", tag, port))
	server, err := net.Listen("tcp", port)
	if err != nil {
		return err
	}
	HandleSignals(server)
	for run {
		// Listen for an incoming connection.
		conn, err := server.Accept()
		if err != nil {
			return err
		}
		lg.Out(fmt.Sprintf("handling new connection from %s", conn.RemoteAddr()))
		go HandleRequest(conn, tag)
	}
	return
}

func HandleRequest(conn net.Conn, tag string) {
	defer conn.Close()
	message, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		lg.Out(fmt.Sprintf("error reading %s", err.Error()))
	}
	if message != "" {
		lg.Out("new message read")
		UbuntuNotify(message, tag)
	}
}

func AnalyseMessage(buff, tag string) (title, message, url string, err error) {
	lg.Out("analysing message")
	split := strings.Split(buff, "|!|")
	lg.Out(string(len(split)))
	if len(split) != 2 {
		err = errors.New(fmt.Sprintf("unable to analyze message %s", buff))
	} else {
		title = fmt.Sprintf("%s: %s", tag, strings.Trim(split[0], " "))
		message = fmt.Sprintf("%s", strings.Trim(split[1], "\n"))
		url = fmt.Sprintf("%s", xurls.Strict().FindString(message))
		lg.Out(fmt.Sprintf("title:\t%s", title))
		lg.Out(fmt.Sprintf("message:\t%s", message))
		lg.Out(fmt.Sprintf("url:\t%s", url))
	}
	return title, message, url, err
}

func UbuntuNotify(buff, tag string) {
	lg.Out("notifiying OSX")
	title, message, _, err := AnalyseMessage(buff, tag)
	if err != nil {
		lg.Error(err)
	} else {
		args := []string{title, message}
		cmd := exec.Command(*flagNotifierCmd, args...)
		lg.Out(fmt.Sprintf("running %s", strings.Join(cmd.Args, " ")))
		err = cmd.Run()
		if err != nil {
			lg.Error(err)
		}
	}
	return
}

func HandleSignals(server net.Listener) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		_ = <-c
		Cleanup(server)
	}()
}

func Cleanup(server net.Listener) {
	lg.Out(fmt.Sprintf("stopping server and cleaning up"))
	run = false
	server.Close()
	pidFile := path.Join(*flagServiceDir, fmt.Sprintf("%s.pid", *flagLink))
	_ = os.Remove(pidFile)
}

func Stop(process *os.Process) {
	lg.Out(fmt.Sprintf("shutting down existing process PID %d", process.Pid))
	process.Signal(syscall.Signal(1))
	os.Exit(0)
}

func main() {
	run = true
	CreateRinoServiceDir()
	if *flagLink == "" {
		kingpin.Usage()
		lg.Fatal(errors.New("rino: error: required flag --link not provided\n"))
	}
	err := SetupPidFile(*flagLink)
	if err != nil {
		lg.Fatal(err)
	}
	split := strings.Split(*flagLink, ":")
	tag := split[0]
	port := fmt.Sprintf(":%s", split[1])
	err = RinoTCPServer(tag, port)
	if err != nil {
		lg.Fatal(err)
	}
	os.Exit(0)
}
