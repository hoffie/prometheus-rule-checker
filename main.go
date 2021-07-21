package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	promql "github.com/prometheus/prometheus/promql/parser"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	verbose = kingpin.Flag("verbose", "Verbose mode.").Short('v').Bool()
	url     = kingpin.Flag("prometheus.url", "prometheus base URL").Required().String()
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

func checkRules() {
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/rules", *url))
	log.WithFields(log.Fields{"resp": resp, "err": err}).Debug("rule query result")
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Rule request failed")
	}

	b, err := io.ReadAll(resp.Body)
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

type visitor struct {
	selectors []string
}

func (v *visitor) Visit(node promql.Node, path []promql.Node) (promql.Visitor, error) {
	if node == nil {
		return v, nil
	}
	log.WithFields(log.Fields{"node": node}).Debug("Visit")
	switch n := node.(type) {
	case *promql.VectorSelector:
		v.selectors = append(v.selectors, n.String())
	default:
		log.Debug("Not handling %T", n)
	}
	return v, nil
}

func checkQuery(query string) error {
	selectors, err := getSelectors(query)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("getSelectors failed")
	}
	log.WithFields(log.Fields{"len(selectors)": len(selectors)}).Debug("Found selectors")

	for _, selector := range selectors {
		log.WithFields(log.Fields{"selector": selector}).Debug("Checking selector")
		c := getResultCount(selector)
		if c < 1 {
			return fmt.Errorf("No results, possibly wrong metric name or impossible selector: %s", selector)
		}
	}
	return nil
}

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

	b, err := io.ReadAll(resp.Body)
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
