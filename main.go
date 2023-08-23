/* Experimental Terminal Parser
 *
 * This is a place to experiment with
 * (1) parsing the output of a terminal and
 * (2) detecting information about the state of the terminal, such as whether an
 *     application is waiting for input or if it wants to be in full-screen mode.
 *
 * In retrospect, I might be wrong about needing to handle (1) here at all. A possible alternate approach
 * would be to stream *mostly* raw program output to the JS client instead of sending pre-rendered HTML.
 * The JS client would handle rendering, while the server would ONLY scan program output for escape sequences
 * that let us do interesting things, like detecting prompts or implementing a pager mode.
 */
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/creack/pty"

	"terminal_parser/terminal"
)

func attachXterm(ptmx *os.File) {
	// `xterm -S<pts>/<fd>` takes two options: the name of a pts and a file descriptor.
	// I can't figure out what the pts is supposed to do, or if it makes a difference.
	//
	// I feel like the upgrade path should probably borrow more state from the terminal than just the ptmx.
	// If we're using something like xterm.js, we would want to:
	// *    forward WINCH events to the running process group (but this could be handled outside the terminal package)
	//
	// Beyond that, we should consider whether we want to support switching BACK from the upgraded terminal
	xtermCmd := exec.Command("xterm", "-S/3")
	xtermCmd.Stdout = os.Stderr
	xtermCmd.Stderr = os.Stderr
	xtermCmd.ExtraFiles = []*os.File{ptmx}
	xtermCmd.Run() // TODO: don't block here?
}

func serveStdout(ptmx *os.File) {
	term := terminal.New(ptmx, terminal.WithUpgradeHook(attachXterm))
	server := http.Server{Addr: "localhost:3000"}

	var seen bool
	waitForBrowser := make(chan struct{})
	http.HandleFunc("/stdout", func(w http.ResponseWriter, req *http.Request) {
		if !seen {
			waitForBrowser <- struct{}{}
			seen = true
		}
		for _, l := range term.Lines() {
			w.Write([]byte(l))
			w.Write([]byte{'\n'})
		}
	})
	http.Handle("/", http.FileServer(http.Dir("web")))
	fmt.Println("Serving stdout on http://localhost:3000")
	go server.ListenAndServe()

	<-waitForBrowser
	term.Run(context.Background())

	seen = false
	<-waitForBrowser
	server.Shutdown(context.Background())
}

func printStdErr(pipe *os.File) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		fmt.Fprintf(os.Stderr, "ERR< %s\n", scanner.Text())
	}
}

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if len(os.Args) < 2 {
		log.Fatal("usage: terminal_parser <command> <args>...")
	}

	ptmx, pts, err := pty.Open()
	if err != nil {
		log.Println(err)
	}

	var rPipe, pipe *os.File
	rPipe, pipe, err = os.Pipe()
	if err != nil {
		log.Println(err)
	}

	waitForOutput := make(chan struct{})
	go func() {
		serveStdout(ptmx)
		waitForOutput <- struct{}{}
	}()
	go func() {
		printStdErr(rPipe)
		waitForOutput <- struct{}{}
	}()

	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdin = pts
	cmd.Stdout = pts
	cmd.Stderr = pipe
	//cmd.Stderr = pts

	err = cmd.Start()
	if err != nil {
		log.Fatal("command failed to start: ", err)
	}
	_ = pipe.Close()
	_ = pts.Close()
	err = cmd.Wait()
	if err != nil {
		log.Print("command exited with err: ", err)
	}

	<-waitForOutput
	<-waitForOutput

	_ = rPipe.Close()
	_ = ptmx.Close()
}
