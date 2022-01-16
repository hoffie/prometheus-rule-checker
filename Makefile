all: build

build:
	go build

livetest-server:
	prometheus --web.listen-address="127.0.0.1:19090" --config.file="livetest/prometheus.yml"

livetest-rules: build
	./prometheus-rule-checker --prometheus.url=http://127.0.0.1:19090
	./prometheus-rule-checker --prometheus.url=http://127.0.0.1:19090 --output.format csv
	./prometheus-rule-checker --prometheus.url=http://127.0.0.1:19090 --output.format json
	./prometheus-rule-checker --prometheus.url=http://127.0.0.1:19090 --output.format human --ignored-selectors.regexp node_filesystem_size_bytes
	./prometheus-rule-checker --prometheus.url=http://127.0.0.1:19090 --output.format human --ignored-selectors.regexp node_filesystem_size_bytes --ignored-selectors.regexp '.*fstype.*'

.PHONY: build livetest-server livetest-rules
