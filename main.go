package main

type PID struct {
	Kp, Ki, Kd        float64
	integral          float64
	previouserror_pid float64
}

// NewPID creates a new PID controller with the specified gains
func NewPID(kp, ki, kd float64) *PID {
	return &PID{
		Kp: kp,
		Ki: ki,
		Kd: kd,
	}
}

// Compute calculates the PID output based on the setpoint and current value
func (pid *PID) Compute(setpoint, currentValue float64) float64 {

	error_pid := setpoint - currentValue

	proportional := pid.Kp * error_pid

	pid.integral += error_pid
	integral := pid.Ki * pid.integral

	derivative := pid.Kd * (error_pid - pid.previouserror_pid)
	pid.previouserror_pid = error_pid

	output := proportional + integral + derivative
	return output
}

func main() {

	QPoc_ref := 0.0
	Pond := 1e6
	QPoc := []float64{Simulation(Pond, 0)}
	Qond := []float64{0}
	pid := NewPID(0.01, 0.1, 0)
	T := []float64{0.0}
	numIterations := 1000
	dt := 0.001

	for t := 1; t < numIterations; t++ {
		Qond = append(Qond, pid.Compute(QPoc_ref, QPoc[len(QPoc)-1]))
		QPoc = append(QPoc, Simulation(Pond, Qond[len(Qond)-1]))
		newT := T[len(T)-1] + dt
		T = append(T, newT)
	}

	Line(T, Qond, "Qond.png")
	Line(T, QPoc, "QPoc.png")
}
