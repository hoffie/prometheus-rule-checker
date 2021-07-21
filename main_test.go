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
		"foo{a=\"1\"} offset 5m": []string{
			"foo{a=\"1\"}",
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

func TestExpandRegexpMatchers(t *testing.T) {
	c := []struct {
		i string
		o []string
	}{
		{
			i: "foo{bar=~\"a|b\"}",
			o: []string{
				"foo{bar=~\"a\"}",
				"foo{bar=~\"b\"}",
			},
		},
		{
			i: "foo{bar!=\"a\",bar=~\"a|b\"}",
			o: []string{
				"foo{bar!=\"a\",bar=~\"a\"}",
				"foo{bar!=\"a\",bar=~\"b\"}",
			},
		},
		{
			i: "foo{a=\"1\",bar=~\"a|b|c\",z=\"2\",splitlater=~\"x|y|z\"}",
			o: []string{
				"foo{a=\"1\",bar=~\"a\",z=\"2\",splitlater=~\"x|y|z\"}",
				"foo{a=\"1\",bar=~\"b\",z=\"2\",splitlater=~\"x|y|z\"}",
				"foo{a=\"1\",bar=~\"c\",z=\"2\",splitlater=~\"x|y|z\"}",
			},
		},
		{
			i: "foo{bar=~\"(a|b|c)\"}",
			o: []string{},
		},
		{
			i: "foo{bar=~\"(a)|b|c\"}",
			o: []string{},
		},
		{
			i: "foo{bar=~\"a\\\\|b|c\"}",
			o: []string{},
		},
		{
			i: "foo{bar=~\"a\"}",
			o: []string{},
		},
		{
			i: "foo{bar=\"a|b|c\"}",
			o: []string{},
		},
		{
			i: "foo{bar!~\"a|b|c\"}",
			o: []string{},
		},
	}
	for _, x := range c {
		mm, err := promql.ParseMetricSelector(x.i)
		if err != nil {
			t.Errorf("%v", err)
		}
		expandeds := expandRegexpMatchers(mm)
		if len(expandeds) != len(x.o) {
			t.Errorf("len of returned vs. expected output mismatch: %d, %d", len(expandeds), len(x.o))
			continue
		}
		for i, expanded := range expandeds {
			expected := x.o[i]
			parsedExpected, err := promql.ParseMetricSelector(expected)
			if err != nil {
				t.Errorf("parsing expected selector failed: %v", err)
			}
			if len(expanded) != len(parsedExpected) {
				t.Errorf("len of returned vs. expected matchers mismatch: %d, %d", len(expanded), len(expected))
				break
			}
			for j := range parsedExpected {
				if expanded[j].String() != parsedExpected[j].String() {
					t.Errorf("item %d mismatched: %v != %v", i, expanded[j], parsedExpected[j])
					t.Errorf("%v %v", expanded, parsedExpected)
					break
				}
			}
		}
	}
}

func TestLabelMatchersToString(t *testing.T) {
	c := []string{
		"foo",
		"foo{bar=\"baz\"}",
		"foo{bar=\"baz\",z=\"1\"}",
	}
	for _, i := range c {
		parsedI, err := promql.ParseMetricSelector(i)
		if err != nil {
			t.Errorf("err: %v", err)
			continue
		}
		o := labelMatchersToString(parsedI)
		if o != i {
			t.Errorf("%v != %v", o, i)
		}
	}
}
