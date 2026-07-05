#!/usr/bin/env ruby
# frozen_string_literal: true

# Consume a WorldSignal subscription by polling — stdlib only.
# Env: WS_API_BASE, WS_API_KEY, WS_SUBSCRIPTION, WS_SINCE, WS_MAX,
#      WS_INTERVAL (seconds, default 3).
require 'json'
require 'net/http'
require 'uri'

BASE = ENV.fetch('WS_API_BASE', 'http://localhost:4000')
KEY = ENV.fetch('WS_API_KEY', '')
SUB = ENV.fetch('WS_SUBSCRIPTION', 'demo-stream')
MAX = ENV.fetch('WS_MAX', '0').to_i
INTERVAL = ENV.fetch('WS_INTERVAL', '3').to_f

abort('WS_API_KEY is required') if KEY.empty?

cursor = ENV.fetch('WS_SINCE', '0').to_i
seen = 0
warn("[poll] #{BASE}/v1/stream/poll subscription=#{SUB}")
loop do
  uri = URI("#{BASE}/v1/stream/poll?subscription=#{SUB}&since=#{cursor}")
  req = Net::HTTP::Get.new(uri)
  req['Authorization'] = "Bearer #{KEY}"
  res = Net::HTTP.start(uri.hostname, uri.port) { |http| http.request(req) }
  body = JSON.parse(res.body)
  (body['events'] || []).each do |ev|
    d = ev.dig('payload', 'data') || {}
    puts format('[poll] %-8s %s  %s', d['severity'] || '?', d['country'] || '--', d['title'] || '')
    seen += 1
    exit(0) if MAX.positive? && seen >= MAX
  end
  cursor = body['cursor'] || cursor
  sleep(INTERVAL)
end
