Output assets/demo.gif
Source assets/tapes/common.tape
Type "printf '2026-10-20 INFO  server started on :8080\n2026-10-20 WARN  cache miss ratio 0.42\n2026-10-20 ERROR db timeout after 30s\nnot a log line\n2026-10-21 INFO  shutdown complete\n' | ./lazyregex-linux --flags gm" Enter
Show
Sleep 1500ms
Type@45ms '^(?P<date>\d{4}-\d{2}-\d{2}) (?P<level>INFO|WARN|ERROR)\s+(?P<msg>.*)$'
Sleep 1500ms
Escape
Type "3"
Sleep 800ms
Enter
Sleep 1500ms
Type "7"
Sleep 800ms
Down
Sleep 700ms
Down
Sleep 700ms
Down
Sleep 700ms
Down
Sleep 1000ms
Type "]"
Sleep 2500ms
