//MIT License
//
//Copyright (c) 2020 sters
//
//Permission is hereby granted, free of charge, to any person obtaining a copy
//of this software and associated documentation files (the "Software"), to deal
//in the Software without restriction, including without limitation the rights
//to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
//copies of the Software, and to permit persons to whom the Software is
//furnished to do so, subject to the following conditions:
//
//The above copyright notice and this permission notice shall be included in all
//copies or substantial portions of the Software.
//
//THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
//IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
//AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
//LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
//OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
//SOFTWARE.
//https://github.com/sters/yaml-diff/blob/master/LICENSE

package util

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sort"
"strings"
	"time"

	"github.com/google/go-cmp/cmp"
)

type DiffStatus int

const (
	DiffStatusExists   DiffStatus = 1
	DiffStatusSame     DiffStatus = 2
	DiffStatus1Missing DiffStatus = 3
	DiffStatus2Missing DiffStatus = 4
)

type RawYaml struct {
	Raw interface{}
	id  string
}

type RawYamlList []*RawYaml

func newRawYaml(raw interface{}) *RawYaml {
	return &RawYaml{
		Raw: raw,
		id:  fmt.Sprintf("%d-%d", time.Now().UnixNano(), randInt()),
	}
}

func randInt() int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(9223372036854775807))
	return n.Int64()
}

type Diff struct {
	Diff      string
	difflines int

	Yaml1Struct *RawYaml
	Yaml2Struct *RawYaml

	Status DiffStatus
}

type Diffs []*Diff

func Do(list1 RawYamlList, list2 RawYamlList) Diffs {
	var result Diffs

	checked := map[string]struct{}{} // RawYaml.id => struct{}

	matchFuncs := []func([]*Diff) *Diff{
		func(diffs []*Diff) *Diff {
			for _, d := range diffs {
				if d.Status == DiffStatusSame {
					return d
				}
			}
			return nil
		},
		func(diffs []*Diff) *Diff {
			sort.Slice(diffs, func(i, j int) bool {
				return diffs[i].difflines < diffs[j].difflines
			})

			return diffs[0]
		},
	}

	for _, matchFunc := range matchFuncs {
		for _, yaml1 := range list1 {
			if _, ok := checked[yaml1.id]; ok {
				continue
			}

			diffs := make([]*Diff, 0, len(list2))

			for _, yaml2 := range list2 {
				if _, ok := checked[yaml2.id]; ok {
					continue
				}

				s := &Diff{
					Diff:        adjustFormat(cmp.Diff(yaml1.Raw, yaml2.Raw)),
					Yaml1Struct: yaml1,
					Yaml2Struct: yaml2,
					Status:      DiffStatusExists,
				}

				if len(strings.TrimSpace(s.Diff)) < 1 {
					s.Status = DiffStatusSame
					s.Diff = createSameFormat(yaml1, s.Status)
				} else {
					for _, str := range strings.Split(s.Diff, "\n") {
						trimmedstr := strings.TrimSpace(str)
						if strings.HasPrefix(trimmedstr, "+") || strings.HasPrefix(str, "-") {
							s.difflines++
						}
					}
				}

				diffs = append(diffs, s)
			}

			if len(diffs) == 0 {
				continue
			}

			d := matchFunc(diffs)
			if d == nil {
				continue
			}

			result = append(result, d)
			checked[d.Yaml1Struct.id] = struct{}{}
			checked[d.Yaml2Struct.id] = struct{}{}
		}
	}

	// check the unmarked items in list1
	for _, yaml1 := range list1 {
		if _, ok := checked[yaml1.id]; ok {
			continue
		}

		result = append(
			result,
			&Diff{
				Diff:        createSameFormat(yaml1, DiffStatus2Missing),
				Yaml1Struct: yaml1,
				Status:      DiffStatus2Missing,
			},
		)
	}

	for _, yaml2 := range list2 {
		if _, ok := checked[yaml2.id]; ok {
			continue
		}

		result = append(
			result,
			&Diff{
				Diff:        createSameFormat(yaml2, DiffStatus1Missing),
				Yaml2Struct: yaml2,
				Status:      DiffStatus1Missing,
			},
		)
	}

	return result
}

func createSameFormat(y *RawYaml, status DiffStatus) string {
	result := strings.Builder{}

	prefix := ""
	switch status {
	case DiffStatusSame:
		prefix = "  "
	case DiffStatus1Missing:
		prefix = "+ "
	case DiffStatus2Missing:
		prefix = "- "
	}

	diff := cmp.Diff(y.Raw, struct{}{})

	for _, str := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(str, "-") {
			continue
		}

		// TODO: cmp.Diff is unstable use custom Reporter
		str = strings.TrimSpace(str)
		str = strings.Replace(str, "- 	", "", 1)
		str = strings.Replace(str, "- 	", "", 1)

		result.WriteString(prefix)
		result.WriteString(str)
		result.WriteRune('\n')
	}

	return adjustFormat(strings.TrimSuffix(result.String(), ",\n")) + "\n"
}

func adjustFormat(s string) string {
	for ss, rr := range map[string]string{
		`map[string]interface{}`: "Map",
		`map[String]interface{}`: "Map",
		`[]interface{}`:          "List",
		`uint64`:                 "Number",
		`int64`:                  "Number",
		`string`:                 "String",
		`bool`:                   "Boolean",
	} {
		s = strings.ReplaceAll(s, ss, rr)
	}

	return s
}

