package preprocessing

import (
	"cmp"
	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/beatmap/objects"
	"github.com/wieku/danser-go/framework/math/vector"
	"slices"
)

const (
	maximumSliderRadius float32 = NormalizedRadius * 2.4
	assumedSliderRadius float32 = NormalizedRadius * 1.8
)

// LazySlider is a utility struct that has LazyEndPosition and LazyTravelDistance needed for difficulty calculations
type LazySlider struct {
	*objects.Slider

	diff *difficulty.Difficulty

	LazyEndPosition    vector.Vector2f
	LazyTravelDistance float32
	LazyTravelTime     float64
}

func NewLazySlider(slider *objects.Slider, d *difficulty.Difficulty) *LazySlider {
	decorated := &LazySlider{
		Slider: slider,
		diff:   d,
	}

	decorated.calculateEndPosition()

	return decorated
}

func (slider *LazySlider) calculateEndPosition() {
	scorePoints := slices.Clone(slider.ScorePointsLazer)

	scorePoints[len(scorePoints)-1].Time = max(slider.StartTime+(slider.EndTimeLazer-slider.StartTime)/2, slider.EndTimeLazer-36)

	slices.SortFunc(scorePoints, func(a, b objects.TickPoint) int {
		return cmp.Compare(a.Time, b.Time)
	})

	slider.LazyTravelTime = scorePoints[len(scorePoints)-1].Time - slider.GetStartTime()

	slider.LazyEndPosition = slider.GetStackedPositionAtModLazer(slider.LazyTravelTime+slider.GetStartTime(), slider.diff) // temporary lazy end position until a real result can be derived.
	currCursorPosition := slider.GetStackedStartPositionMod(slider.diff)
	scalingFactor := NormalizedRadius / slider.diff.CircleRadiusU // lazySliderDistance is coded to be sensitive to scaling, this makes the maths easier with the thresholds being used.

	for i := 0; i < len(scorePoints); i++ {
		var currMovementObj = scorePoints[i]

		var stackedPosition vector.Vector2f
		if currMovementObj.LastPoint { // bug that made into deployment but well
			stackedPosition = slider.GetStackedPositionAtModLazer(slider.EndTimeLazer, slider.diff)
		} else {
			stackedPosition = slider.GetStackedPositionAtModLazer(currMovementObj.Time, slider.diff)
		}

		currMovement := stackedPosition.Sub(currCursorPosition)
		currMovementLength := scalingFactor * float64(currMovement.Len())

		// Amount of movement required so that the cursor position needs to be updated.
		requiredMovement := float64(assumedSliderRadius)

		if i == len(scorePoints)-1 {
			// The end of a slider has special aim rules due to the relaxed time constraint on position.
			// There is both a lazy end position as well as the actual end slider position. We assume the player takes the simpler movement.
			// For sliders that are circular, the lazy end position may actually be farther away than the sliders true end.
			// This code is designed to prevent buffing situations where lazy end is actually a less efficient movement.
			lazyMovement := slider.LazyEndPosition.Sub(currCursorPosition)

			if lazyMovement.Len() < currMovement.Len() {
				currMovement = lazyMovement
			}

			currMovementLength = scalingFactor * float64(currMovement.Len())
		} else if currMovementObj.IsReverse {
			// For a slider repeat, assume a tighter movement threshold to better assess repeat sliders.
			requiredMovement = NormalizedRadius
		}

		if currMovementLength > requiredMovement {
			// this finds the positional delta from the required radius and the current position, and updates the currCursorPosition accordingly, as well as rewarding distance.
			currCursorPosition = currCursorPosition.Add(currMovement.Scl(float32((currMovementLength - requiredMovement) / currMovementLength)))
			currMovementLength *= (currMovementLength - requiredMovement) / currMovementLength
			slider.LazyTravelDistance += float32(currMovementLength)
		}

		if i == len(scorePoints)-1 {
			slider.LazyEndPosition = currCursorPosition
		}
	}
}
