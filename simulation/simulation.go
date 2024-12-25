package simulation

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
func (pid *PID) Compute(setpoint, currentValue, dt float64) float64 {

	error_pid := setpoint - currentValue

	proportional := pid.Kp * error_pid

	pid.integral += error_pid * dt
	integral := pid.Ki * pid.integral

	derivative := pid.Kd * (error_pid - pid.previouserror_pid) / dt
	pid.previouserror_pid = error_pid

	output := proportional + integral + derivative
	return output
}

func Simulation(Sp, Tau, K, P, Ki, Kd, dt, N float64) ([]float64, []float64) {

	measure := []float64{0}
	T := []float64{0}

	pid := NewPID(P, Ki, Kd)

	var un float64

	for k := 1; k <= int(N); k++ {
		un = pid.Compute(Sp, measure[len(measure)-1], dt)
		ynn := DynamicResponse(un, measure[len(measure)-1], dt, Tau, K)
		measure = append(measure, ynn)
		T = append(T, T[len(T)-1]+dt)
	}

	return T, measure
}

func DynamicResponse(un, yn, dt, Tau, K float64) float64 {
	return (dt/Tau)*(K*un-yn) + yn
}
