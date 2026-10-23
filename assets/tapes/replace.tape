Output assets/replace.gif
Source assets/tapes/common.tape
Type "printf '2026-10-20 INFO  server started on :8080\n2026-10-20 WARN  cache miss ratio 0.42\n2026-10-20 ERROR db timeout after 30s\nnot a log line\n2026-10-21 INFO  shutdown complete\n' | ./lazyregex-linux --flags gm -p '^(?P<date>\d{4}-\d{2}-\d{2}) (?P<level>INFO|WARN|ERROR)\s+(?P<msg>.*)$'" Enter
Show
Sleep 2000ms
Escape
Sleep 600ms
Type "]"
Sleep 1000ms
Enter
Sleep 400ms
Down
Sleep 500ms
Type@80ms '[\g<level>] \g<date> — \g<msg>'
Sleep 3500ms
