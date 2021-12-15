package main

import (
	"fmt"
	"math"
)

func main() {
	const TYPE = Classic
	const USER = "person2597"

	replays, err := GetReplays(USER)
	if err != nil {
		panic(err)
	}
	replays = ApplyFilter(FilterType(TYPE), replays)
	xStat := StatisticNumber{}
	yStat := StatisticAverage{100, StatisticPercentile{}}
	x := GetStat(xStat, replays, USER)
	y := GetStat(yStat, replays, USER)

	fmt.Printf("%s,\"%s\",%s\n", StringifyStat(xStat), StringifyStat(yStat), TYPE)
	for i := len(y) - 1; i >= 0; i-- {
		if !math.IsNaN(x[i]) && !math.IsNaN(y[i]) {
			fmt.Printf("%f,%f,\n", x[i], y[i])
		}
	}
}
