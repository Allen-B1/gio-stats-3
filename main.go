package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"html/template"
	"math"
	"sort"
	"strconv"
)

/*
func main() {
	const TYPE = Classic
	const USER = "person2597"

	replays, err := GetReplays(USER)
	if err != nil {
		panic(err)
	}
	replays = ApplyFilter(FilterType(TYPE), replays)
	xStat := StatisticNumber{}
	yStat := StatisticAverage{25, StatisticPercentile{}}
	x := GetStat(xStat, replays, USER)
	y := GetStat(yStat, replays, USER)

	fmt.Printf("%s,\"%s\",%s\n", StringifyStat(xStat), StringifyStat(yStat), TYPE)
	for i := len(y) - 1; i >= 0; i-- {
		if !math.IsNaN(x[i]) && !math.IsNaN(y[i]) {
			fmt.Printf("%f,%f,\n", x[i], y[i])
		}
	}
}
*/

// [x, y] is top corner; [w, h] is size of graph
func transform(point [2]float64, min [2]float64, max [2]float64, x, y, w, h int) [2]float64 {
	return [2]float64{
		((point[0]-min[0])/(max[0]-min[0]))*float64(w) + float64(x),
		((max[1]-point[1])/(max[1]-min[1]))*float64(h) + float64(y),
	}
}

func makeLine(xdata []float64, ydata []float64, min, max [2]float64, x, y, w, h int, color string) string {
	fmt.Println(len(xdata))
	pairs := make([][2]float64, 0)
	for i, _ := range xdata {
		if !math.IsNaN(xdata[i]) && !math.IsNaN(ydata[i]) {
			pairs = append(pairs, [2]float64{xdata[i], ydata[i]})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i][0] == pairs[j][0] {
			return pairs[i][1] < pairs[j][1]
		}
		return pairs[i][0] < pairs[j][0]
	})

	//	data := `<svg viewBox="0 0 500 500" xmlns="http://www.w3.org/2000/svg">`

	data := ""
	for i, point1 := range pairs {
		if i == len(pairs)-1 {
			break
		}
		point2 := pairs[i+1]
		new1 := transform(point1, min, max, x, y, w, h)
		new2 := transform(point2, min, max, x, y, w, h)
		data += fmt.Sprintf(`<line x1="%f" y1="%f" x2="%f" y2="%f" stroke-width="2" stroke="%s" />`, new1[0], new1[1], new2[0], new2[1], color)
	}
	//	data += "</svg>"
	return data
}

func main() {
	r := gin.New()
	r.LoadHTMLFiles("tmpl/stat.html", "tmpl/header.html", "tmpl/nav.html")
	r.GET("/stats", func(c *gin.Context) {
		username := c.Query("username")
		xstr := c.Query("x")
		ystr := c.Query("y")
		gametype := c.Query("type")

		xstat, err := ParseStat(xstr)
		if err != nil {
			c.String(400, err.Error())
			return
		}
		ystat, err := ParseStat(ystr)
		if err != nil {
			c.String(400, err.Error())
			return
		}

		replays, err := GetReplays(username)
		if err != nil {
			c.String(400, err.Error())
			return
		}
		replays = ApplyFilter(FilterType(gametype), replays)

		if len(replays) == 0 {
			c.String(400, "no data")
			return
		}

		xdata := GetStat(xstat, replays, username)
		ydata := GetStat(ystat, replays, username)

		max := [2]float64{math.Inf(-1), math.Inf(-1)}
		min := [2]float64{math.Inf(1), math.Inf(1)}

		for _, x := range xdata {
			if x > max[0] {
				max[0] = x
			}
			if x < min[0] {
				min[0] = x
			}
		}
		for _, y := range ydata {
			if y > max[1] {
				max[1] = y
			}
			if y < min[1] {
				min[1] = y
			}
		}

		chart := template.HTML(fmt.Sprintf(`<svg viewBox="0 0 %d %d" width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">`, 768+128, 512+128, 768+128, 512+128))
		chart += template.HTML(makeLine(xdata, ydata, min, max, 64, 64, 768, 512, "#1133ff"))

		chart += template.HTML(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#111" stroke-width="4" />`, 64, 512+64, 768+64, 512+64))
		chart += template.HTML(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#111" stroke-width="4" />`, 64, 64, 64, 512+64))

		// calculate label positions for x
		diff := math.Pow(10, float64(int(math.Log10(max[0]-min[0]))))
		if (max[0]-min[0])/float64(diff) <= 6 {
			diff /= 2
		}

		step := float64(int(min[0]/diff)) * diff

		for step < max[0] {
			x := ((step-min[0])/(max[0]-min[0]))*float64(768) + 64
			chart += template.HTML(fmt.Sprintf(`<text x="%f" y="%f" text-anchor="middle">`, x, float64(512+96)) + strconv.FormatFloat(step, 'G', 3, 64) + `</text>`)
			step += diff
		}

		// calculate label positions for y
		diff = math.Pow(10, float64(int(math.Log10(max[1]-min[1]))))
		if (max[1]-min[1])/float64(diff) <= 6 {
			diff /= 2
		}
		if (max[1]-min[1])/float64(diff) <= 5 {
			diff /= 2
		}

		step = float64(int(min[1]/diff)) * diff

		for step < max[0] {
			y := ((max[1]-step)/(max[1]-min[1]))*float64(512) + 64
			chart += template.HTML(fmt.Sprintf(`<text x="%f" y="%f" text-anchor="middle">`, float64(32), y) + strconv.FormatFloat(step, 'G', 3, 64) + `</text>`)
			step += diff
		}

		chart += "</svg>"

		c.HTML(200, "stat.html", gin.H{"Username": username, "XStat": StringifyStat(xstat), "YStat": StringifyStat(ystat), "Chart": chart})
	})
	r.Run()
}
