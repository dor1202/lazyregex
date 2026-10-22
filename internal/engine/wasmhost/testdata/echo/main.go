// Command echo is a wasip1 test engine speaking the NDJSON protocol.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type req struct {
	ID      int    `json:"id"`
	Op      string `json:"op"`
	Pattern string `json:"pattern"`
	Text    string `json:"text"`
}

func main() {
	// ECHO_START_DELAY (a time.Duration) simulates a slow-starting engine.
	if d, err := time.ParseDuration(os.Getenv("ECHO_START_DELAY")); err == nil {
		time.Sleep(d)
	}
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1<<20), 1<<30)
	out := bufio.NewWriter(os.Stdout)
	for sc.Scan() {
		var r req
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			fmt.Fprintln(os.Stderr, "bad request:", err)
			os.Exit(2)
		}
		switch r.Op {
		case "ping":
			fmt.Fprintf(out, "{\"id\":%d,\"ok\":true,\"version\":\"echo-1\"}\n", r.ID)
		case "eval":
			if r.Pattern == "hang" {
				for {
				}
			}
			if r.Pattern == "crash" {
				fmt.Fprintln(os.Stderr, "boom")
				os.Exit(3)
			}
			b, _ := json.Marshal(map[string]any{"id": r.ID, "ok": true, "output": strings.ToUpper(r.Text)})
			out.Write(b)
			out.WriteByte('\n')
		}
		out.Flush()
	}
}
