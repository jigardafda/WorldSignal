<?php
// Consume a WorldSignal subscription by polling — no extensions required.
// Env: WS_API_BASE, WS_API_KEY, WS_SUBSCRIPTION, WS_SINCE, WS_MAX,
//      WS_INTERVAL (seconds, default 3).

$base = getenv('WS_API_BASE') ?: 'http://localhost:4000';
$key = getenv('WS_API_KEY') ?: '';
$sub = getenv('WS_SUBSCRIPTION') ?: 'demo-stream';
$max = (int)(getenv('WS_MAX') ?: '0');
$interval = (float)(getenv('WS_INTERVAL') ?: '3');

if ($key === '') { fwrite(STDERR, "WS_API_KEY is required\n"); exit(2); }

$cursor = (int)(getenv('WS_SINCE') ?: '0');
$seen = 0;
fwrite(STDERR, "[poll] $base/v1/stream/poll subscription=$sub\n");
while (true) {
    $ctx = stream_context_create(['http' => [
        'method' => 'GET',
        'header' => "Authorization: Bearer $key\r\n",
    ]]);
    $raw = file_get_contents("$base/v1/stream/poll?subscription=$sub&since=$cursor", false, $ctx);
    $body = json_decode($raw, true);
    foreach (($body['events'] ?? []) as $ev) {
        $d = $ev['payload']['data'] ?? [];
        printf("[poll] %-8s %s  %s\n", $d['severity'] ?? '?', $d['country'] ?? '--', $d['title'] ?? '');
        if ($max > 0 && ++$seen >= $max) { exit(0); }
    }
    $cursor = $body['cursor'] ?? $cursor;
    usleep((int)($interval * 1_000_000));
}
