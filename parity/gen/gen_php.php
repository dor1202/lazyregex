<?php
// Runs parity corpus cases through PHP's real PCRE2 (preg_*).
// Reads a JSON array of cases on stdin, writes a JSON array of results to stdout.

function pcre_error(string $warning, string $fallback): string {
    // "preg_match(): Compilation failed: ... at offset 3" -> drop the function prefix.
    $msg = preg_replace('/^preg_\w+\(\): /', '', $warning);
    return $msg !== '' ? $msg : $fallback;
}

function span_of($cap): array {
    // $cap is [string|null, offset] (PREG_OFFSET_CAPTURE | PREG_UNMATCHED_AS_NULL).
    if ($cap[0] === null) {
        return [false, [-1, -1]];
    }
    return [true, [$cap[1], $cap[1] + strlen($cap[0])]];
}

function match_entry(array $m): array {
    [, $sp] = span_of($m[0]);
    $groups = [];
    $name = '';
    foreach ($m as $k => $cap) {
        if ($k === 0) {
            continue;
        }
        if (is_string($k)) {
            $name = $k;
            continue;
        }
        [$matched, $span] = span_of($cap);
        $groups[] = ['index' => $k, 'name' => $name, 'matched' => $matched, 'span' => $span];
        $name = '';
    }
    return ['span' => $sp, 'groups' => $groups];
}

function run_case(array $c): array {
    $res = ['id' => $c['id'], 'ok' => true, 'matches' => [], 'unit' => 'bytes'];
    $warning = '';
    set_error_handler(function ($no, $str) use (&$warning) {
        $warning = $str;
        return true;
    });
    $fail = false;
    $flags = $c['flags'] ?? '';
    $glob = strpos($flags, 'g') !== false;
    $mods = str_replace('g', '', $flags);
    $rx = '/' . addcslashes($c['pattern'], '/') . '/' . $mods;
    $text = $c['text'];
    $mode = ($c['mode'] ?? '') ?: 'match';
    $fl = PREG_OFFSET_CAPTURE | PREG_UNMATCHED_AS_NULL;
    if ($mode === 'replace') {
        $out = preg_replace($rx, $c['replacement'] ?? '', $text, $glob ? -1 : 1);
        if ($out === null) { $fail = true; } else { $res['output'] = $out; }
    } elseif ($mode === 'split') {
        $parts = preg_split($rx, $text);
        if ($parts === false) { $fail = true; } else { $res['parts'] = $parts; }
    } elseif ($glob) {
        $r = preg_match_all($rx, $text, $sets, $fl | PREG_SET_ORDER);
        if ($r === false) { $fail = true; } else {
            foreach ($sets as $m) { $res['matches'][] = match_entry($m); }
        }
    } else {
        $r = preg_match($rx, $text, $m, $fl);
        if ($r === false) { $fail = true; } elseif ($r === 1) { $res['matches'][] = match_entry($m); }
    }
    restore_error_handler();
    if ($fail) {
        return ['id' => $c['id'], 'ok' => false,
            'error_message' => pcre_error($warning, preg_last_error_msg()),
            'matches' => [], 'unit' => ''];
    }
    return $res;
}

$cases = json_decode(stream_get_contents(STDIN), true);
$out = array_map('run_case', $cases);
echo json_encode($out, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES | JSON_INVALID_UTF8_SUBSTITUTE | JSON_PARTIAL_OUTPUT_ON_ERROR);
