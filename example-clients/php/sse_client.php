<?php
// Consume a WorldSignal subscription as Server-Sent Events — no extensions.
// Opens the stream with an Authorization header and parses `data:` lines.
// Env: WS_API_BASE, WS_API_KEY, WS_SUBSCRIPTION, WS_SINCE, WS_MAX.

$base = getenv('WS_API_BASE') ?: 'http://localhost:4800';
$key = getenv('WS_API_KEY') ?: '';
$sub = getenv('WS_SUBSCRIPTION') ?: 'demo-stream';
$since = getenv('WS_SINCE') ?: '0';
$max = (int)(getenv('WS_MAX') ?: '0');

if ($key === '') { fwrite(STDERR, "WS_API_KEY is required\n"); exit(2); }

$url = "$base/v1/stream/sse?subscription=$sub&since=$since";
fwrite(STDERR, "[sse] connecting to $url\n");
$ctx = stream_context_create(['http' => [
    'method' => 'GET',
    'header' => "Authorization: Bearer $key\r\n",
    'timeout' => 30,
]]);
$fp = fopen($url, 'r', false, $ctx);
if ($fp === false) { fwrite(STDERR, "[sse] connection failed\n"); exit(1); }

$seen = 0;
while (($line = fgets($fp)) !== false) {
    $line = rtrim($line, "\r\n");
    if (strncmp($line, 'data:', 5) !== 0) { continue; } // skip id:/event:/comments
    $ev = json_decode(trim(substr($line, 5)), true);
    $d = $ev['data'] ?? [];
    printf("[sse] %-8s %s  %s\n", $d['severity'] ?? '?', $d['country'] ?? '--', $d['title'] ?? '');
    if ($max > 0 && ++$seen >= $max) {
        fwrite(STDERR, "[sse] received $seen event(s), exiting\n");
        fclose($fp);
        exit(0);
    }
}
fclose($fp);
