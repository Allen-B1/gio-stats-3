package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

type GameType string

const (
	Classic GameType = "classic"
	M1v1    GameType = "1v1"
	M2v2    GameType = "2v2"
	Custom  GameType = "custom"
)

type Ranking struct {
	Name        string `json:"name"`
	Stars       uint8  `json:"stars"`
	CurrentName string `json:"currentName"`
}

type ReplayEntry struct {
	Type    GameType  `json:"type"`
	ID      string    `json:"id"`
	Started uint64    `json:"started"`
	Turn    uint32    `json:"turn"`
	Ranking []Ranking `json:"ranking"`
}

func GetReplays(username string) ([]*ReplayEntry, error) {
	var allEntries []*ReplayEntry
	for i := 0; ; i += 200 {
		resp, err := http.Get("https://generals.io/api/replaysForUsername?u=" + url.QueryEscape(username) + "&offset=" + fmt.Sprint(i) + "&count=200")
		if err != nil {
			return nil, err
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var entries []*ReplayEntry = nil
		err = json.Unmarshal(body, &entries)
		if err != nil {
			return nil, err
		}

		if len(entries) == 0 {
			break
		}

		allEntries = append(allEntries, entries...)
	}
	return allEntries, nil
}

type Filter interface {
	Matches(entry *ReplayEntry) bool
}

// Type FilterType represents a filter based on game type
// (classic, 1v1, 2v2, custom).
type FilterType GameType

func (f FilterType) Matches(entry *ReplayEntry) bool {
	return entry.Type == GameType(f)
}

type FilterAgainst string

func (f FilterAgainst) Matches(entry *ReplayEntry) bool {
	for _, ranking := range entry.Ranking {
		if ranking.CurrentName == string(f) {
			return true
		}
	}
	return false
}

type FilterAnd []Filter

func (f FilterAnd) Matches(entry *ReplayEntry) bool {
	for _, filter := range f {
		if !filter.Matches(entry) {
			return false
		}
	}
	return true
}

type FilterOr []Filter

func (f FilterOr) Matches(entry *ReplayEntry) bool {
	for _, filter := range f {
		if filter.Matches(entry) {
			return true
		}
	}
	return false
}

type Statistic interface {
	For(entries []*ReplayEntry, index int, username string) float64
}

// Type StatisticWin returns 1 for a winning game and 0 for a non-winning game.
type StatisticWin struct{}

func won2v2(entry *ReplayEntry, username string) bool {
	winningStars := entry.Ranking[0].Stars

	// if two teams have different stars, winning team has `winningStars`
	// if everyone has same stars, estimate winning team as the first two players

	if entry.Ranking[1].Stars == winningStars && entry.Ranking[2].Stars == winningStars { // same stars
		if entry.Ranking[0].CurrentName == username || entry.Ranking[1].CurrentName == username {
			return true
		}
		return false
	} else { // different stars
		for _, ranking := range entry.Ranking {
			if ranking.CurrentName == username && ranking.Stars == winningStars {
				return true
			}
		}
		return false
	}
}

func (s StatisticWin) For(entries []*ReplayEntry, index int, username string) float64 {
	entry := entries[index]
	switch entry.Type {
	case Classic, Custom, M1v1:
		if entry.Ranking[0].CurrentName == username {
			return 1
		}
		return 0
	case M2v2:
		if won2v2(entry, username) {
			return 1
		}
		return 0
	}

	panic("unknown game type")
}

type StatisticStars struct{}

func (s StatisticStars) For(entries []*ReplayEntry, index int, username string) float64 {
	entry := entries[index]
	for _, ranking := range entry.Ranking {
		if ranking.CurrentName == username {
			return float64(ranking.Stars)
		}
	}
	return math.NaN()
}

type StatisticPercentile struct{}

func (s StatisticPercentile) For(entries []*ReplayEntry, index int, username string) float64 {
	entry := entries[index]
	switch entry.Type {
	case Classic, Custom:
		for i := 0; i < len(entry.Ranking); i++ {
			if entry.Ranking[i].CurrentName == username {
				return float64(len(entry.Ranking)-i-1) / float64(len(entry.Ranking)-1)
			}
		}
		return 0
	case M1v1:
		if entry.Ranking[0].CurrentName == username {
			return 1
		}
		return 0
	case M2v2:
		if won2v2(entry, username) {
			return 1
		}
		return 0
	}
	panic("invalid game type: " + string(entry.Type))
}

// StatisticAverage is a weighted average of the match +-N (N*2 surrounding matches)
type StatisticAverage struct {
	N  int
	Of Statistic
}

func (s StatisticAverage) For(entries []*ReplayEntry, index int, username string) float64 {
	totalValue := float64(0)
	totalWeight := float64(0)
	for i := -s.N; i < s.N; i++ {
		idx := index + i
		if idx < 0 || idx >= len(entries) {
			continue
		}
		value := s.Of.For(entries, idx, username)
		if !math.IsNaN(value) {
			weight := float64(i) / float64(s.N)
			weight = 1 - weight*weight
			//			log.Println(idx, weight, len(entries))
			totalValue += value * weight
			totalWeight += weight
		}
	}
	return totalValue / totalWeight
}

var _ = log.Println

type StatisticNumber struct{}

func (s StatisticNumber) For(entries []*ReplayEntry, index int, username string) float64 {
	return float64(len(entries) - index)
}

type StatisticDate struct{}

func (s StatisticDate) For(entries []*ReplayEntry, index int, username string) float64 {
	return float64(entries[index].Started)
}

func ApplyFilter(filter Filter, entries []*ReplayEntry) []*ReplayEntry {
	out := make([]*ReplayEntry, 0, len(entries)/2)
	for _, entry := range entries {
		if filter.Matches(entry) {
			out = append(out, entry)
		}
	}
	return out
}

func GetStat(stat Statistic, entries []*ReplayEntry, username string) []float64 {
	out := make([]float64, len(entries))
	for i, _ := range entries {
		out[i] = stat.For(entries, i, username)
	}
	return out
}

func StringifyStat(stat Statistic) string {
	value := reflect.ValueOf(stat)
	name := value.Type().Name()
	if strings.HasPrefix(name, "Statistic") {
		name = name[len("Statistic"):]
	}

	if value.Type().Kind() == reflect.Struct {
		l := value.NumField()
		if l == 0 {
			return name
		}
		fields := make([]string, 0)
		for i := 0; i < l; i++ {
			v := value.Field(i)
			if s, ok := v.Interface().(Statistic); ok {
				fields = append(fields, StringifyStat(s))
			} else {
				fields = append(fields, fmt.Sprint(v.Interface()))
			}
		}

		return name + "[" + strings.Join(fields, ",") + "]"
	} else {
		return name + "[" + fmt.Sprint(stat) + "]"
	}
}
