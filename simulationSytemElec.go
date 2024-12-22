package main

import (
	"fmt"
	"math"
)

// ElectricalSystem represents the parameters of the electrical system
type ElectricalSystem struct {
	L    float64 // Inductance in henrys
	C    float64 // Capacitance in farads
	R    float64 // Resistance in ohms
	f    float64 // Frequency in hertz
	UPoc float64
}

// ComputeImpedance calculates the impedance of the system
func (sys *ElectricalSystem) ComputeImpedance() (float64, float64) {

	X_L := 2 * math.Pi * sys.f * sys.L

	X_C := 1 / (2 * math.Pi * sys.f * sys.C)

	Z := math.Sqrt(math.Pow(sys.R, 2) + math.Pow(X_L-X_C, 2))
	theta := math.Atan2(X_L-X_C, sys.R)

	return Z, theta
}

// ComputeReactivePower calculates the reactive power Q based on the applied active power P
func (sys *ElectricalSystem) ComputeReactivePowerSys(I float64) float64 {

	X_L := 2 * math.Pi * sys.f * sys.L
	var X_C float64

	if sys.C == 0 {
		X_C = 0
	} else {
		X_C = 1 / (2 * math.Pi * sys.f * sys.C)
	}

	Q_L := math.Pow(I, 2) * X_L
	Q_C := math.Pow(I, 2) * X_C

	Q := Q_L - Q_C

	return Q
}

func (sys *ElectricalSystem) ComputeS(Pond, Qond float64) float64 {
	S := math.Sqrt(math.Pow(Pond, 2) + math.Pow(Qond, 2))
	fmt.Println("S =", S)
	return S

}

func (sys *ElectricalSystem) ComputeQPoc(Pond, Qond float64) float64 {

	S := sys.ComputeS(Pond, Qond)
	I := S / sys.UPoc
	Qsys := sys.ComputeReactivePowerSys(I)
	Qpoc := Qond + Qsys
	fmt.Println("QPoc = ", Qpoc)
	return Qpoc

}

func Simulation(Pond, Qond float64) float64 {
	// Define the parameters of the electrical system
	sys := ElectricalSystem{
		L:    2.8e-3, // Inductance in henrys
		C:    0,      // Capacitance in farads
		R:    0,      // Resistance in ohms
		f:    50,     // Frequency in hertz
		UPoc: 6700,
	}
	QPoc := sys.ComputeQPoc(Pond, Qond)
	return QPoc
}
