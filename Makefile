all: build

build:
	go build

livetest-server:
	prometheus --web.listen-address="127.0.0.1:19090" --config.file="livetest/prometheus.yml"

livetest-rules: build
	./prometheus-rule-checker --prometheus.url=http://127.0.0.1:19090 | grep -P "(Selectors with no results:|- node_filesystem_free_bytes.*fstype|- node_filesystem_size_bytes$$)" | wc -l | grep -qx 3
	./prometheus-rule-checker --prometheus.url=http://127.0.0.1:19090 --output.format csv | grep -P 'File;Group;Name;Query;Problematic selector|livetest/rules.yml;base;FilesystemFilled;node_filesystem_free_bytes.fstype!~"tmpfs.rpc_pipefs". / node_filesystem_size_bytes . 100 < 15;node_filesystem_free_bytes.fstype!~"tmpfs.rpc_pipefs".|livetest/rules.yml;base;FilesystemFilled;node_filesystem_free_bytes.fstype!~"tmpfs.rpc_pipefs". / node_filesystem_size_bytes . 100 < 15;node_filesystem_size_bytes' | wc -l | grep -qx 3
	./prometheus-rule-checker --prometheus.url=http://127.0.0.1:19090 --output.format json | grep -P '^\[$$|^\]$$|"Name": "FilesystemFilled",' | wc -l | grep -qx 3
	./prometheus-rule-checker --prometheus.url=http://127.0.0.1:19090 --output.format human --ignored-selectors.regexp node_filesystem_size_bytes | grep -v node_filesystem_size_bytes >/dev/null
	./prometheus-rule-checker --prometheus.url=http://127.0.0.1:19090 --output.format human --ignored-selectors.regexp node_filesystem_size_bytes --ignored-selectors.regexp '.*fstype.*' # expect exit code 0
	./prometheus-rule-checker --prometheus.url=http://127.0.0.1:19090 || true # expect exit code 1

.PHONY: build livetest-server livetest-rules
