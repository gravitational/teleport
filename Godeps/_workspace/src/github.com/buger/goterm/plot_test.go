package goterm

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
)

func TestCreateDataTable(t *testing.T) {
	data := new(DataTable)

	data.AddColumn("Gender")
	data.AddColumn("Age")

	if len(data.columns) != 2 {
		t.Error("Should be 2 columns")
	}

	if data.columns[1] != "Age" {
		t.Error("Should have proper column name")
	}

	data.AddRow(1, 5)
	data.AddRow(0, 4)

	if len(data.rows) != 2 {
		t.Error("Should have 2 rows")
	}

	if data.rows[1][0] != 0 && data.rows[1][1] != 4 {
		t.Error("Row should be properly inserted")
	}
}

func TestLineChartIndependent(t *testing.T) {
	fmt.Println("Independent charts\n")

	chart := NewLineChart(100, 20)
	chart.Flags = DRAW_INDEPENDENT //| DRAW_RELATIVE

	data := new(DataTable)
	data.AddColumn("Time")
	data.AddColumn("Lat")
	data.AddColumn("Count")

	//data.AddColumn("x*x")

	for i := 0; i < 60; i++ {
		data.AddRow(float64(i+60), float64(20+rand.Intn(10)), float64(i*2+rand.Intn(i+1))) // ,*/, x*x)
	}

	fmt.Println(chart.Draw(data))
}

func TestLineChartRelative(t *testing.T) {
	fmt.Println("Relative chart\n")

	chart := NewLineChart(100, 20)
	chart.Flags = DRAW_RELATIVE

	data := new(DataTable)
	data.AddColumn("X")
	data.AddColumn("Sin(x)")
	data.AddColumn("Cos(x+1)")

	//data.AddColumn("x*x")

	for i := 0.1; i < 10; i += 0.1 {
		data.AddRow(i, math.Sin(i), math.Cos(i+1))
	}

	fmt.Println(chart.Draw(data))
}

func TestLineChart(t *testing.T) {
	fmt.Println("Simple chart\n")

	chart := NewLineChart(100, 20)
	//chart.Flags = /*DRAW_INDEPENDENT // | */// DRAW_RELATIVE

	data := new(DataTable)
	data.AddColumn("x")
	data.AddColumn("fx1")
	data.AddColumn("fx2")

	for i := -5.0; i < 5; i += 0.1 {
		data.AddRow(i, 3*math.Sin(i)+3*i+30, i*i+5)
	}

	fmt.Println(chart.Draw(data))
}
