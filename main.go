package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/prometheus/pkg/labels"
	promql "github.com/prometheus/prometheus/promql/parser"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	verbose  = kingpin.Flag("verbose", "Verbose mode.").Short('v').Bool()
	url      = kingpin.Flag("prometheus.url", "prometheus base URL").Required().String()
	waitTime = kingpin.Flag("wait.seconds", "seconds to wait between count requests").Default("0.01").Float()
)

func main() {
	kingpin.Parse()
	if *verbose {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	log.WithFields(log.Fields{"prometheus.url": *url}).Debug("Querying")

	checkRules()
}

// checkRules is the main entry point, connects to the Prometheus API, retrieves all defined rules and analyzes the PromQL expressions for dead metric references.
func checkRules() {
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/rules", *url))
	log.WithFields(log.Fields{"resp": resp, "err": err}).Debug("rule query result")
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Rule request failed")
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Body reading failed")
	}

	j := struct {
		Status string
		Data   struct {
			Groups []struct {
				Name  string
				File  string
				Rules []struct {
					Name  string
					Query string
				}
			}
		}
	}{}
	err = json.Unmarshal(b, &j)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("json parsing failed")
	}

	if j.Status != "success" {
		log.WithFields(log.Fields{"status": j.Status}).Fatal("Unexpected status in rule request")
	}

	for _, g := range j.Data.Groups {
		for _, r := range g.Rules {
			log.WithFields(log.Fields{"group": g.Name, "file": g.File, "name": r.Name, "query": r.Query}).Debug("Checking rule")
			err := checkQuery(r.Query)
			if err != nil {
				log.WithFields(log.Fields{"group": g.Name, "file": g.File, "name": r.Name, "query": r.Query, "err": err}).Warn("Potentially broken rule")
			}
		}
	}
}

// visitor struct is used to collect selectors from a PromQL expression.
type visitor struct {
	selectors []string
}

// Visit is called by promql.Walk when traversing a PromQL expression's syntax tree.
func (v *visitor) Visit(node promql.Node, path []promql.Node) (promql.Visitor, error) {
	if node == nil {
		return v, nil
	}
	log.WithFields(log.Fields{"node": node}).Debug("Visit")
	switch n := node.(type) {
	case *promql.VectorSelector:
		v.selectors = append(v.selectors, n.String())
	default:
		log.Debugf("Not handling %T", n)
	}
	return v, nil
}

// checkQuery parses the given query and ensures that all contained
// selectors yield results by querying the Prometheus API.
func checkQuery(query string) error {
	selectors, err := getSelectors(query)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("getSelectors failed")
	}
	log.WithFields(log.Fields{"len(selectors)": len(selectors)}).Debug("Found selectors")

	var selector string
	for len(selectors) > 0 {
		selector, selectors = selectors[0], selectors[1:]
		log.WithFields(log.Fields{"selector": selector}).Debug("Checking selector")
		matchers, err := promql.ParseMetricSelector(selector)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Fatal("Metric selector parsing failed")
		}
		if ignoreMatchers(matchers) {
			log.WithFields(log.Fields{"selector": selector}).Debug("Not checking ignored metric")
			break
		}
		expanded := expandRegexpMatchers(matchers)
		if len(expanded) != 0 {
			for _, e := range expanded {
				selectors = append(selectors, labelMatchersToString(e))
			}
			continue
		}
		time.Sleep(time.Duration(*waitTime) * time.Second)
		c := getResultCount(selector)
		if c < 1 {
			return fmt.Errorf("No results, possibly wrong metric name or impossible selector: %s", selector)
		}
	}
	return nil
}

// ignoreMatchers returns true if the given metric should be
// ignored.
func ignoreMatchers(matchers []*labels.Matcher) bool {
	for _, m := range matchers {
		log.WithFields(log.Fields{"m": m}).Debug("Matcher")
		if m.Name != "__name__" {
			continue
		}
		if m.Value == "ALERTS" || m.Value == "ALERTS_FOR_STATE" {
			// Those are temporary, internal metrics which may generate
			// false positives.
			return true
		}
	}
	return false
}

// expandRegexpMatchers walks the given list of label matchers and
// attempts to expand the first found expandable regexp matcher.
// A regexp matcher is expandable if it is a simple list of alternatives in the
// form a|b|c.
// foo=~"a|b|c" will be turned into
//   foo=~"a"
//   foo=~"b"
//   foo=~"c"
// This function is supposed to handle the common, trivial case. It is not
// able to handle arbitrarily complex regexp.
func expandRegexpMatchers(matchers []*labels.Matcher) [][]*labels.Matcher {
	expanded := make([][]*labels.Matcher, 0)
	var alternatives []string
	for _, m := range matchers {
		if m.Type != labels.MatchRegexp {
			continue
		}
		if strings.ContainsAny(m.Value, "()\\") {
			continue
		}
		alternatives = strings.Split(m.Value, "|")
		if len(alternatives) < 2 {
			continue
		}
		candidateLabel := m.Name
		for _, alt := range alternatives {
			e := make([]*labels.Matcher, len(matchers))
			for i, n := range matchers {
				var mCopy labels.Matcher
				if n.Name == candidateLabel {
					n.Value = alt
				}
				mCopy = *n
				e[i] = &mCopy
			}
			expanded = append(expanded, e)
		}
		return expanded
	}
	return expanded
}

// getResultCount queries the Prometheus API and counts the number of results
// for the given selector.
func getResultCount(selector string) uint64 {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/query", *url), nil)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Query request init failed")
	}
	count := fmt.Sprintf("count(%s)", selector)
	q := req.URL.Query()
	q.Add("query", count)
	req.URL.RawQuery = q.Encode()
	client := http.Client{}
	resp, err := client.Do(req)
	log.WithFields(log.Fields{"resp": resp, "err": err}).Debug("rule query result")
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Rule request failed")
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Body reading failed")
	}

	j := struct {
		Status string
		Data   struct {
			Result []struct {
				Metric map[string]string
				Value  []interface{}
			}
		}
	}{}
	err = json.Unmarshal(b, &j)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("json parsing failed")
	}

	if j.Status != "success" {
		log.WithFields(log.Fields{"status": j.Status}).Fatal("Unexpected status in rule request")
	}
	if len(j.Data.Result) != 1 {
		return 0
	}
	i, err := strconv.ParseUint(j.Data.Result[0].Value[1].(string), 10, 64)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Int conversion failed")
	}
	return i
}

// getSelectors parses the given PromQL query and extracts
// all selectors.
// Example:
//   foo{a="1"} > bar{b="2"}
// yields
//   foo{a="1"}
//   bar{a="2"}
func getSelectors(query string) ([]string, error) {
	expr, err := promql.ParseExpr(query)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Debug("ParseExpr")
		return nil, fmt.Errorf("promql parse error: %s", err)
	}
	log.Debug(promql.Tree(expr))
	v := &visitor{
		selectors: make([]string, 0),
	}
	var path []promql.Node
	promql.Walk(v, expr, path)
	return v.selectors, nil
}

// labelMatchersToString takes a list of label matchers
// and re-constructs a VectorSelector in string format from it.
func labelMatchersToString(lms []*labels.Matcher) string {
	name := ""
	for _, lm := range lms {
		if lm.Name == labels.MetricName {
			name = lm.Value
		}
	}
	vs := promql.VectorSelector{
		Name:          name,
		LabelMatchers: lms,
	}
	return vs.String()
}
