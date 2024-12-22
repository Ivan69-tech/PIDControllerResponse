package main

import (
	"fmt"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

func Line(X []float64, Y []float64, name string) {
	if len(X) != len(Y) {
		fmt.Errorf("Erreur dans le tracé, X et Y ne sont pas de la même taille")
	}

	points := make(plotter.XYs, len(X))
	for i := range X {
		points[i].X = float64(X[i])
		points[i].Y = Y[i]
	}

	p := plot.New()

	p.Title.Text = "Plot des données X et Y"
	p.X.Label.Text = "X"
	p.Y.Label.Text = "Y"

	line, err := plotter.NewLine(points)
	if err != nil {
		panic(err)
	}

	p.Add(line)

	if err := p.Save(8*vg.Inch, 4*vg.Inch, name); err != nil {
		panic(err)
	}
}
