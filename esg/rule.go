// Copyright (c) 2019, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package esg

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/emer/emergent/erand"
	"github.com/goki/ki/kit"
)

// RuleTypes are different types of rules (i.e., how the items are selected)
type RuleTypes int32

//go:generate stringer -type=RuleTypes

var KiT_RuleTypes = kit.Enums.AddEnum(RuleTypesN, kit.NotBitFlag, nil)

func (ev RuleTypes) MarshalJSON() ([]byte, error)  { return kit.EnumMarshalJSON(ev) }
func (ev *RuleTypes) UnmarshalJSON(b []byte) error { return kit.EnumUnmarshalJSON(ev, b) }

const (
	// UniformItems is the default mutually exclusive items chosen at uniform random
	UniformItems RuleTypes = iota

	// ProbItems has specific probabilities for each item
	ProbItems

	// CondItems has conditionals for each item, indicated by ?
	CondItems

	// SequentialItems progresses through items in sequential order, indicated by |
	SequentialItems

	// PermutedItems progresses through items in permuted order, indicated by $
	PermutedItems

	RuleTypesN
)

// Rule is one rule containing some number of items
type Rule struct {
	Name   string    `desc:"name of rule"`
	Desc   string    `desc:"description / notes on rule"`
	Type   RuleTypes `desc:"type of rule -- how to choose the items"`
	Items  []*Item   `desc:"items in rule"`
	State  State     `desc:"state update for rule"`
	CurIdx int       `desc:"current index in Items, if sequential or permuted"`
	Order  []int     `desc:"permuted order if doing that"`
}

// Init initializes the rules -- only relevant for ordered rules (restarts at start)
func (rl *Rule) Init() {
	rl.CurIdx = 0
	if rl.Type == PermutedItems {
		rl.Order = rand.Perm(len(rl.Items))
	}
}

// Gen generates expression according to the rule
func (rl *Rule) Gen(rls *Rules) []string {
	rls.SetFired(rl.Name)
	rl.State.Set(rls, rl.Name)
	if rls.Trace {
		fmt.Printf("Fired Rule: %v\n", rl.Name)
	}
	switch rl.Type {
	case UniformItems:
		no := len(rl.Items)
		opt := rand.Intn(no)
		if rls.Trace {
			fmt.Printf("Selected item: %v from: %v uniform random\n", opt, no)
		}
		return rl.Items[opt].Gen(rl, rls)
	case ProbItems:
		pv := rand.Float32()
		sum := float32(0)
		for ii, it := range rl.Items {
			sum += it.Prob
			if pv < sum { // note: lower values already excluded
				if rls.Trace {
					fmt.Printf("Selected item: %v using rnd val: %v sum: %v\n", ii, pv, sum)
				}
				return it.Gen(rl, rls)
			}
		}
		if rls.Trace {
			fmt.Printf("No items selected using rnd val: %v sum: %v\n", pv, sum)
		}
		return nil
	case CondItems:
		var copts []int
		for ii, it := range rl.Items {
			if it.CondTrue(rl, rls) {
				copts = append(copts, ii)
			}
		}
		no := len(copts)
		if no == 0 {
			if rls.Trace {
				fmt.Printf("No items match Conds\n")
			}
			return nil
		}
		opt := rand.Intn(no)
		if rls.Trace {
			fmt.Printf("Selected item: %v from: %v matching Conds\n", copts[opt], no)
		}
		return rl.Items[copts[opt]].Gen(rl, rls)
	case SequentialItems:
		no := len(rl.Items)
		if no == 0 {
			return nil
		}
		if rl.CurIdx >= no {
			rl.CurIdx = 0
		}
		opt := rl.CurIdx
		if rls.Trace {
			fmt.Printf("Selected item: %v sequentially\n", opt)
		}
		rl.CurIdx++
		return rl.Items[opt].Gen(rl, rls)
	case PermutedItems:
		no := len(rl.Items)
		if no == 0 {
			return nil
		}
		if len(rl.Order) != no {
			rl.Order = rand.Perm(no)
			rl.CurIdx = 0
		}
		if rl.CurIdx >= no {
			erand.PermuteInts(rl.Order)
			rl.CurIdx = 0
		}
		opt := rl.Order[rl.CurIdx]
		if rls.Trace {
			fmt.Printf("Selected item: %v sequentially\n", opt)
		}
		rl.CurIdx++
		return rl.Items[opt].Gen(rl, rls)
	}
	return nil
}

// String generates string representation of rule
func (rl *Rule) String() string {
	if strings.HasSuffix(rl.Name, "SubRule") {
		str := " {\n"
		for _, it := range rl.Items {
			str += "\t\t" + it.String() + "\n"
		}
		str += "\t}\n"
		return str
	} else {
		str := "\n\n"
		if rl.Desc != "" {
			str += "// " + rl.Desc + "\n"
		}
		str += rl.Name
		switch rl.Type {
		case CondItems:
			str += " ? "
		case SequentialItems:
			str += " | "
		case PermutedItems:
			str += " $ "
		}
		str += " {\n"
		for _, it := range rl.Items {
			str += "\t" + it.String() + "\n"
		}
		str += "}\n"
		return str
	}
}

// Validate checks for config errors
func (rl *Rule) Validate(rls *Rules) []error {
	nr := len(rl.Items)
	if nr == 0 {
		err := fmt.Errorf("Rule: %v has no items", rl.Name)
		return []error{err}
	}
	var errs []error
	for _, it := range rl.Items {
		if rl.Type == CondItems {
			if len(it.Cond) == 0 {
				errs = append(errs, fmt.Errorf("Rule: %v is CondItems, but Item: %v has no Cond", rl.Name, it.String()))
			}
			if it.SubRule == nil {
				errs = append(errs, fmt.Errorf("Rule: %v is CondItems, but Item: %v has nil SubRule", rl.Name, it.String()))
			}
		} else {
			if rl.Type == ProbItems && it.Prob == 0 {
				errs = append(errs, fmt.Errorf("Rule: %v is ProbItems, but Item: %v has 0 Prob", rl.Name, it.String()))
			} else if rl.Type == UniformItems && it.Prob > 0 {
				errs = append(errs, fmt.Errorf("Rule: %v is UniformItems, but Item: %v has > 0 Prob", rl.Name, it.String()))
			}
		}
		iterrs := it.Validate(rl, rls)
		if len(iterrs) > 0 {
			errs = append(errs, iterrs...)
		}
	}
	return errs
}
