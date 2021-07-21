package main

import (
	"reflect"
	"testing"

	promql "github.com/prometheus/prometheus/promql/parser"
)

func TestIgnoreMatchers(t *testing.T) {
	c := map[string]bool{
		"ALERTS{foo=\"bar\"}":           true,
		"ALERTS_FOR_STATE{foo=\"bar\"}": true,
		"ALERTS_X{foo=\"bar\"}":         false,
		"up{foo=\"bar\"}":               false,
	}
	for q, i := range c {
		m, err := promql.ParseMetricSelector(q)
		if err != nil {
			panic(err)
		}
		if ignoreMatchers(m) != i {
			t.Errorf("%s not ignored", q)
		}
	}
}

func TestGetSelectors(t *testing.T) {
	c := map[string][]string{
		"foo{a=\"1\"} > on(instance) bar{a=\"1\"}": []string{
			"foo{a=\"1\"}",
			"bar{a=\"1\"}",
		},
	}
	for q, e := range c {
		r, err := getSelectors(q)
		if err != nil {
			t.Errorf("%v", err)
		}
		if !reflect.DeepEqual(r, e) {
			t.Errorf("%s: %v != %v", q, r, e)
		}
	}
}
