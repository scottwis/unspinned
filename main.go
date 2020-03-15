package main

import (
	"flag"
	"fmt"
	"github.com/scottwis/unspinned/theSkyX"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultStepSize theSkyX.Degrees = 0.001
	defaultHost = "localhost"
	defaultPort = "3040"
	defaultTrackingRate = "sidereal"
)

var trackingRates = map[string]theSkyX.Radians{
	"sidereal": theSkyX.Radians(0.00007292115),
	"lunar":    theSkyX.Radians(0.0000711948891),
	"solar":    theSkyX.Radians(0.0000727221),
}

type cmdValues struct {
	trackingRate string
	host string
	port string
	stepSize string
}

func formatTrackingRates() string {
	var b strings.Builder

	first := true
	for key := range trackingRates {
		if ! first {
			b.Write([]byte(", "))
		} else {
			first = false
		}
		_, _ = fmt.Fprintf(&b, "'%v'", key)
	}

	return b.String()
}

func main() {
	var values cmdValues

	flag.StringVar(
		&values.trackingRate,
		"rate",
		"",
		fmt.Sprintf(
			"The tracking rate to use. Defaults to '%[1]v'. You should use '%[1]v' for most objects. Valid values are %[2]v, or a custom floating point value in radians / second.",
			defaultTrackingRate,
			formatTrackingRates(),
		),
	)

	flag.StringVar(
		&values.host,
		"host",
		"",
		fmt.Sprintf("The host running TSX to connect to. Defaults to '%v'.", defaultHost),
	)

	flag.StringVar(
		&values.port,
		"port",
		"",
		fmt.Sprintf("The port to connect to. Defaults to '%v'.", defaultPort),
	)

	flag.StringVar(
		&values.stepSize,
		"stepSize",
		"",
		fmt.Sprintf("The rotator step size in degrees. Defaults to '%v'", defaultStepSize),
	)

	flag.Parse()

	if values.host == "" {
		values.host = defaultHost
	}

	if values.port == "" {
		values.port = defaultPort
	}

	if values.trackingRate == "" {
		values.trackingRate = defaultTrackingRate
	}

	if values.stepSize == "" {
		values.stepSize = string(float64(defaultStepSize))
	}

	tsx, err := theSkyX.New(values.host, values.port)

	if err != nil {
		fmt.Printf("error connecting to The Skyx on '%v:%v': %v\n", values.host, values.port, err)
		os.Exit(-1)
	}

	defer tsx.Close()

	var totalAngularDelta theSkyX.Degrees
	var comunicatedDelta theSkyX.Degrees
	last := time.Now()
	current := last
	trackingRate, err := computeTrackingRate(values.trackingRate)

	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	stepSize, err := strconv.ParseFloat(values.stepSize, 64)
	if err != nil {
		fmt.Printf("Invalid step size: %v\n", values.stepSize)
		os.Exit(-1)
	}

	state, err := tsx.GetState()

	if err != nil {
		fmt.Printf("error comunicating with TSX: %v\n", err)
		os.Exit(-1)
	}

	for {
		// -trackingRate * cos(az)*cos(lat)/cos(alt)
		rotationRate := theSkyX.Radians(
			float64(-trackingRate) *
			math.Cos(float64(state.PointingAt.Az.ToRadians())) *
			math.Cos(float64(state.Latitude.ToRadians())) /
			math.Cos(float64(state.PointingAt.Alt.ToRadians())),
		)

		newDelta := (rotationRate * theSkyX.Radians(current.Sub(last).Seconds())).ToDegrees()

		totalAngularDelta += newDelta

		if math.Abs(float64(totalAngularDelta - comunicatedDelta)) > stepSize {
			deltaToComunicate := (totalAngularDelta - comunicatedDelta) -
				theSkyX.Degrees(math.Mod(float64(totalAngularDelta - comunicatedDelta), stepSize))

			state, err = tsx.Rotate(state.RotatorAngle + deltaToComunicate)

			if err != nil {
				if ! strings.HasSuffix(err.Error(), "A Rotator command is already in progress.") {
					fmt.Printf("error comunicating with TSX: %v\n", err)
					os.Exit(-1)
				}
			} else {
				comunicatedDelta += deltaToComunicate
				fmt.Printf(
					"rotated %v degress (%v total), rotation rate = %v deg/sec, trackingRate = %v, current == %v\n",
					deltaToComunicate,
					comunicatedDelta,
					rotationRate.ToDegrees(),
					values.trackingRate, state.RotatorAngle,
				)
			}
		}

		last, current = current, time.Now()
	}
}

func computeTrackingRate(rate string) (theSkyX.Radians, error) {
	ret, ok := trackingRates[rate]
	if ! ok {
		f, err := strconv.ParseFloat(rate, 64)
		if err != nil {
			return 0.0, fmt.Errorf("invalid tracking rate '%v'", rate)
		}
		return theSkyX.Radians(f), nil
	}
	return ret, nil
}