package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

const (
	APP_NAME    = "rino"
	APP_VERSION = "0.1"
	APP_AUTHOR  = "bigo@crisidev.org"
	APP_SITE    = "https://github.com/crisidev/rino"
)

var (
	versionFlag bool
	pidFile     string
	logFile     string
	debugFlag   bool
	linkFlag    string
	command     string
	notifier    string
	rinoWg      sync.WaitGroup
	rinoDir     = fmt.Sprintf("%s/.rino", os.Getenv("HOME"))
	rinoLinks   = make(RinoLinks)
)

// Init functions
func init() {
	flag.BoolVar(&versionFlag, "version", false, "Print the version number and exit.")
	flag.BoolVar(&versionFlag, "V", false, "Print the version number and exit (shorthand)")

	flag.BoolVar(&debugFlag, "debug", false, "Run in debug mode only for fist container.")
	flag.BoolVar(&debugFlag, "d", false, "Run in debug mode only for fist container (shothand).")

	flag.StringVar(&pidFile, "pidfile", fmt.Sprintf("%s/rino.pid", rinoDir), "Path of the pid file in daemon mode.")
	flag.StringVar(&pidFile, "p", fmt.Sprintf("%s/rino.pid", rinoDir), "Path of the pid file in daemon mode (shorthand).")

	flag.StringVar(&logFile, "logfile", fmt.Sprintf("%s/rino.log", rinoDir), "Path of the log file in daemon mode.")
	flag.StringVar(&logFile, "l", fmt.Sprintf("%s/rino.log", rinoDir), "Path of the log file in daemon mode (shorthand).")
	flag.StringVar(&linkFlag, "link", "BIGO:4223", "Link rino with a running SSH process, giving a tag like \"BIGO:4223\" where BIGO\n\t\twill be the name printed as identifier by terminal-notifier and 4223 is the local port\n\t\tforwarded via SSH which rino with bind to.\n\t\tThis can be specified for more than one SSH connection, using a tag like \"BIGO:4222,ANNA:4223\".")
	flag.StringVar(&linkFlag, "L", "BIGO:4223", "Link rino with a running SSH process, giving a tag like \"BIGO:4223\" where BIGO\n\t\twill be the name printed as identifier by terminal-notifier and 4223 is the local port\n\t\tforwarded via SSH which rino with bind to.\n\t\tThis can be specified for more than one SSH connection, using a tag like \"BIGO:4222,ANNA:4223\" (shorthand).")

	flag.StringVar(&command, "command", "status", "Command to sent do daemon (Needs a valid link).")
	flag.StringVar(&command, "c", "status", "Command to sent do daemon (Needs a valid link) (shorthand).")

	flag.StringVar(&notifier, "notifier", "/usr/local/bin/terminal-notifier", "Terminal notifier path.")
	flag.StringVar(&notifier, "n", "/usr/local/bin/terminal-notifier", "Terminal notifier path (shorthand).")
}

// RinoLinks map
type RinoLinks map[string]*RinoLink

type RinoLink struct {
	port string
	ctrl chan int
}

func (this RinoLinks) Split(links string) {
	for _, v := range strings.Split(links, ",") {
		link := strings.Split(v, ":")
		this[link[0]] = &RinoLink{port: fmt.Sprintf(":%s", link[1]), ctrl: make(chan int)}
	}
}

// Routines handling functions
func routineUp() {
	rinoWg.Add(1)
}

func routineDown(tag string) {
	rinoLinks[tag].ctrl <- 1
}

func disconnect(port string) {
	conn, _ := net.Dial("tcp", fmt.Sprintf("127.0.0.1%s", port))
	conn.Write([]byte("Bye"))
}

func routinesDown() {
	log.Printf("tearing down goroutines, this could take a while...")
	for k, v := range rinoLinks {
		go routineDown(k)
		disconnect(v.port)
	}
}

func handleSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		_ = <-c
		routinesDown()
	}()
}

// Utility functions
func setupLogging() {
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("error opening the log file", err)
	}
	log.SetOutput(file)
}

func stripChars(str, delimiter string) string {
	return strings.Map(func(r rune) rune {
		if strings.IndexRune(delimiter, r) < 0 {
			return r
		}
		return -1
	}, str)
}

// Rino Functions
func rinoTCPServer(port string) net.Listener {
	log.Printf("starting listener on %s", port)
	// Close the listener when the application closes.
	// Listen for incoming connections.
	server, err := net.Listen("tcp", port)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	return server
}

func rinoDaemon(tag string, rinoLink RinoLink) {
	server := rinoTCPServer(rinoLink.port)
	defer server.Close()
	for {
		select {
		case <-rinoLink.ctrl:
			rinoWg.Done()
			log.Printf("goroutine for %s%s stopped", tag, rinoLink.port)
			return
		default:
			// Listen for an incoming connection.
			conn, err := server.Accept()
			if err != nil {
				log.Fatalln("error accepting tcp connection", err.Error())
			}
			routineUp()
			go handleRequest(conn, tag)
		}
	}
}

func handleRequest(conn net.Conn, tag string) {
	defer func() {
		conn.Close()
		rinoWg.Done()
	}()
	// Make a buffer to hold incoming data.
	buf := make([]byte, 2048)
	// Read the incoming connection into the buffer.
	readLen, err := conn.Read(buf)
	if err != nil {
		log.Println("error reading", err.Error())
	}
	if readLen > 0 {
		routineUp()
		go osxNotify(buf, tag)
	}
}

// Notification functions
type RinoMsg struct {
	username string
	message  string
}

func (this *RinoMsg) Read(message []byte) {
	split := strings.Split(stripChars(string(message), "\n"), "|x|")
	if len(split) == 1 {
		this.username = "root"
		this.message = split[0]
	} else {
		this.username = split[0]
		this.message = split[1]
	}
}

func printCommand(cmd *exec.Cmd) {
	fmt.Printf("==> Executing: %s\n", strings.Join(cmd.Args, " "))
}

func printError(err error) {
	if err != nil {
		os.Stderr.WriteString(fmt.Sprintf("==> Error: %s\n", err.Error()))
	}
}

func printOutput(outs []byte) {
	if len(outs) > 0 {
		fmt.Printf("==> Output: %s\n", string(outs))
	}
}

func osxNotify(message []byte, tag string) {
	defer rinoWg.Done()
	msg := &RinoMsg{}
	msg.Read(message)
	if _, err := os.Stat(notifier); err == nil {
		cmd := exec.Command(notifier, "-message", msg.username, "-title", fmt.Sprintf("%s: %s", tag, msg.username), "-sender", "com.apple.iChat")
		err := cmd.Run()
		if err != nil {
			log.Println("error executing terminal-notifier", err.Error())
		}
	}
}

// Main
func main() {
	flag.Parse()
	handleSignals()
	if debugFlag == false {
		setupLogging()
	}
	rinoLinks.Split(linkFlag)
	for k, v := range rinoLinks {
		routineUp()
		go rinoDaemon(k, *v)
	}
	rinoWg.Wait()
}
