# prometheus-rule-checker
This tool connects to a Prometheus server, retrieves all defined rules, parses the expression, takes each referenced metric selector and queries it for existence.
It tries to help with the question: "Can this alert ever fire?" and is supposed to complement unit tests.

## Why would I use it?
- It may detect references to missing metrics due to unnoticed changed exporters
- It may detect wrong label matchers such as regexps in non-regexp matchers.

## Build
This tool is built using Go (tested with 1.16 or newer).
Dependencies have been vendored (using `go mod vendor`) to allow for reproducible builds and simplified cloning.

`go get -u github.com/hoffie/prometheus-rule-checker`

## Configuration
prometheus-rule-checker is configured using command line options only (see `--help` and the example below):

```bash
$ ./prometheus-rule-checker --prometheus.url 127.0.0.1:9090
```

The **default output format** is *human*.
It can be switched to CSV via `--output.format csv` and to JSON via `--output.format json` to simplify integration into CI pipelines.

Known false-positives no-result selectors can be **ignored** by specifying them in a `--ignored-selectors.regexp`.
This option can be repeated.

Query workload on the target prometheus server can be reduced by increasing the **interval between queries** via `--wait.seconds 1.5`.

Regexp based matchers can usually not be tested individually.
However, one common special case is handled explicitly:
Rules such as `up{instance=~"a|b|c"}` can be analyzed for each regexp alternative group (i.e. `a`, `b`, `c`) by enabling this feature with `--expand.regexps`.

More logging can be enabled by specifying `--verbose`.


## License
This software is released under the [Apache 2.0 license](LICENSE).

## Author
prometheus-rule-checker has been created by [Christian Hoffmann](https://hoffmann-christian.info/).
If you find this project useful, please star it or drop me a short [mail](mailto:mail@hoffmann-christian.info).
