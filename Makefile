all: build

build:
	go build

livetest-server:
	prometheus --web.listen-address="127.0.0.1:19090" --config.file="livetest/prometheus.yml"

livetest-rules: build
	./prometheus-rule-checker --prometheus.url=http://127.0.0.1:19090
	./prometheus-rule-checker --prometheus.url=http://127.0.0.1:19090 --output.format json

.PHONY: build livetest-server livetest-rules
