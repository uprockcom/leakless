package leakless

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os/exec"
	"time"

	"github.com/ysmood/leakless/pkg/shared"
	"github.com/ysmood/leakless/pkg/utils"
)

// Launcher struct
type Launcher struct {
	// Lock for leakless.LockPort, default is 2978
	Lock int

	pid chan int
	err string
}

// New leakless instance
func New() *Launcher {
	return &Launcher{
		Lock: 2978,
		pid:  make(chan int),
	}
}

// Command will try to download the leakless bin and prefix the exec.Cmd with the leakless options.
func (l *Launcher) Command(name string, arg ...string) *exec.Cmd {
	bin := ""
	func() {
		defer LockPort(l.Lock)()
		bin = GetLeaklessBin()
	}()

	uid := fmt.Sprintf("%x", utils.RandBytes(16))
	addr := l.serve(uid)

	arg = append([]string{uid, addr, name}, arg...)
	return exec.Command(bin, arg...)
}

// Pid signals the pid of the guarded sub-process. The channel may never receive the pid.
func (l *Launcher) Pid() chan int {
	return l.pid
}

// Err message from the guard process
func (l *Launcher) Err() string {
	return l.err
}

func (l *Launcher) serve(uid string) string {
	srv, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic("[leakless] serve error: " + err.Error())
	}

	go func() {
		defer func() { _ = srv.Close() }()

		conn, err := srv.Accept()
		if err != nil {
			l.err = err.Error()
			l.pid <- 0
			return
		}

		enc := json.NewEncoder(conn)
		err = enc.Encode(shared.Message{UID: uid})
		if err != nil {
			l.err = err.Error()
			l.pid <- 0
			return
		}

		dec := json.NewDecoder(conn)
		var msg shared.Message
		err = dec.Decode(&msg)
		if err == nil {
			l.err = msg.Error
			l.pid <- msg.PID
		}
		_ = dec.Decode(&msg)
	}()

	return srv.Addr().String()
}

// leaklessBin holds the path to a pre-built leakless executable.
var leaklessBin string

// SetLeaklessBin sets the path for the leakless executable.
func SetLeaklessBin(path string) {
	leaklessBin = path
}

// GetLeaklessBin returns the executable path of the guard.
func GetLeaklessBin() string {
	return leaklessBin
}

// Support returns true if the OS is supported by leakless.
func Support() bool {
	return true
}

// LockPort uses a tcp port to create a mutex lock for cross-process locking.
// It will poll the port to check if it's free.
func LockPort(port int) func() {
	var l net.Listener
	for {
		var err error
		l, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			break
		}
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
	}

	return func() {
		_ = l.Close()
	}
}
