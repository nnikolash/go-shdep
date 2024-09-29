package shdep_test

import (
	"testing"

	"github.com/nnikolash/go-shdep"
	"github.com/stretchr/testify/require"
)

func TestEventAccum_Basic(t *testing.T) {
	t.Parallel()

	acc := shdep.NewEventsPullStorage[int]()
	puller := acc.NewPuller()
	acc.Publish(1)
	acc.Publish(2)

	require.Equal(t, 2, acc.Len())

	events := puller.Pull()
	require.Equal(t, 2, len(events))
	require.Equal(t, 1, *events[0].Event)
	require.Equal(t, 2, *events[1].Event)

	require.Equal(t, 0, acc.Len())
}

func TestEventAccum_MultiplePullers(t *testing.T) {
	t.Parallel()

	acc := shdep.NewEventsPullStorage[int]()
	puller1 := acc.NewPuller()
	puller2 := acc.NewPuller()
	acc.Publish(1)
	acc.Publish(2)

	require.Equal(t, 2, acc.Len())

	events1 := puller1.Pull()
	require.Equal(t, 2, len(events1))
	require.Equal(t, 1, *events1[0].Event)
	require.Equal(t, 2, *events1[1].Event)

	events1 = puller1.Pull()
	require.Equal(t, 0, len(events1))

	require.Equal(t, 2, acc.Len())

	events2 := puller2.Pull()
	require.Equal(t, 2, len(events2))
	require.Equal(t, 1, *events2[0].Event)
	require.Equal(t, 2, *events2[1].Event)

	require.Equal(t, 0, acc.Len())

	events1 = puller1.Pull()
	require.Equal(t, 0, len(events1))

	events2 = puller2.Pull()
	require.Equal(t, 0, len(events2))
}

func TestEventAccum_MultiplePullersMultiplePulls(t *testing.T) {
	t.Parallel()

	acc := shdep.NewEventsPullStorage[int]()
	puller1 := acc.NewPuller()
	puller2 := acc.NewPuller()
	acc.Publish(1)
	acc.Publish(2)

	require.Equal(t, 2, acc.Len())

	events1 := puller1.Pull()
	require.Equal(t, 2, len(events1))
	require.Equal(t, 1, *events1[0].Event)
	require.Equal(t, 2, *events1[1].Event)

	require.Equal(t, 2, acc.Len())

	acc.Publish(3)
	acc.Publish(4)

	require.Equal(t, 4, acc.Len())

	events2 := puller2.Pull()
	require.Equal(t, 4, len(events2))
	require.Equal(t, 1, *events2[0].Event)
	require.Equal(t, 2, *events2[1].Event)
	require.Equal(t, 3, *events2[2].Event)
	require.Equal(t, 4, *events2[3].Event)

	require.Equal(t, 2, acc.Len())

	events2 = puller2.Pull()
	require.Equal(t, 0, len(events2))

	events1 = puller1.Pull()
	require.Equal(t, 2, len(events1))
	require.Equal(t, 3, *events1[0].Event)
	require.Equal(t, 4, *events1[1].Event)

	require.Equal(t, 0, acc.Len())

}
