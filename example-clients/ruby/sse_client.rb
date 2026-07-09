#!/usr/bin/env ruby
# frozen_string_literal: true

# Consume a WorldSignal subscription as Server-Sent Events — stdlib only.
# Reads the stream incrementally and parses `data:` lines, using the
# Authorization header (EventSource can't set one). Env: WS_API_BASE,
# WS_API_KEY, WS_SUBSCRIPTION, WS_SINCE, WS_MAX.
require 'json'
require 'net/http'
require 'uri'

BASE = ENV.fetch('WS_API_BASE', 'http://localhost:4800')
KEY = ENV.fetch('WS_API_KEY', '')
SUB = ENV.fetch('WS_SUBSCRIPTION', 'demo-stream')
SINCE = ENV.fetch('WS_SINCE', '0')
MAX = ENV.fetch('WS_MAX', '0').to_i

abort('WS_API_KEY is required') if KEY.empty?

uri = URI("#{BASE}/v1/stream/sse?subscription=#{SUB}&since=#{SINCE}")
warn("[sse] connecting to #{uri}")
req = Net::HTTP::Get.new(uri)
req['Authorization'] = "Bearer #{KEY}"

seen = 0
buf = +''
Net::HTTP.start(uri.hostname, uri.port) do |http|
  http.request(req) do |res|
    res.read_body do |chunk|
      buf << chunk
      while (nl = buf.index("\n"))
        line = buf.slice!(0..nl).chomp
        next unless line.start_with?('data:')

        ev = JSON.parse(line[5..].strip)
        d = ev['data'] || {}
        puts format('[sse] %-8s %s  %s', d['severity'] || '?', d['country'] || '--', d['title'] || '')
        seen += 1
        if MAX.positive? && seen >= MAX
          warn("[sse] received #{seen} event(s), exiting")
          exit(0)
        end
      end
    end
  end
end
