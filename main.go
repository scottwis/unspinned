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

const epsilon theSkyX.Degrees = 0.001

type cmdValues struct {
	trackingRate string
	host string
	port string
}

func main() {
	var values cmdValues

	flag.StringVar(
		&values.trackingRate,
		"rate",
		"",
		"The tracking rate to use. Defaults to 'sidereal'. You should use 'sidereal' for most objects. Valid values are 'sidereal', 'lunar', 'solar', or a custom value. For custom values, enter a floating point value in radians / second.",
	)

	flag.StringVar(
		&values.trackingRate,
		"host",
		"",
		"The host running TSX to connect to. Defaults to 'localhost'.",
	)

	flag.StringVar(&values.port, "port", "", "The port to connect to. Defaults to '3040'.")

	flag.Parse()

	if values.host == "" {
		values.host = "localhost"
	}

	if values.port == "" {
		values.port = "3040"
	}

	if values.trackingRate == "" {
		values.trackingRate = "sidereal"
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
		fmt.Printf("invalid tracking rate: %v\n", err)
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

		if math.Abs(float64(totalAngularDelta - comunicatedDelta)) > float64(epsilon) {
			deltaToComunicate := (totalAngularDelta - comunicatedDelta) -
				theSkyX.Degrees(math.Mod(float64(totalAngularDelta - comunicatedDelta), float64(epsilon)))

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
	switch rate {
	case "sidereal":
		return theSkyX.Radians(0.00007292115), nil
	case "lunar":
		return theSkyX.Radians(0.0000711948891), nil
	case "solar":
		return theSkyX.Radians(0.0000727221), nil
	default:
		rate, err := strconv.ParseFloat(rate, 64)
		if err != nil {
			return 0.0, err
		}
		return theSkyX.Radians(rate), err
	}
}