package updtree_test

import (
	"testing"

	"github.com/nnikolash/go-shdep/updtree"
	"github.com/stretchr/testify/require"
)

func TestEventAccum_Basic(t *testing.T) {
	t.Parallel()

	publisher := updtree.NewEventsPullStorage[int]()
	puller := publisher.NewPuller()
	publisher.Publish(1)
	publisher.Publish(2)

	require.Equal(t, 2, publisher.Len())

	events := puller.Pull()
	require.Equal(t, 2, len(events))
	require.Equal(t, 1, *events[0].Event)
	require.Equal(t, 2, *events[1].Event)

	require.Equal(t, 0, publisher.Len())
}

func TestEventAccum_MultiplePullers(t *testing.T) {
	t.Parallel()

	publisher := updtree.NewEventsPullStorage[int]()
	puller1 := publisher.NewPuller()
	puller2 := publisher.NewPuller()
	publisher.Publish(1)
	publisher.Publish(2)

	require.Equal(t, 2, publisher.Len())

	events1 := puller1.Pull()
	require.Equal(t, 2, len(events1))
	require.Equal(t, 1, *events1[0].Event)
	require.Equal(t, 2, *events1[1].Event)

	events1 = puller1.Pull()
	require.Equal(t, 0, len(events1))

	require.Equal(t, 2, publisher.Len())

	events2 := puller2.Pull()
	require.Equal(t, 2, len(events2))
	require.Equal(t, 1, *events2[0].Event)
	require.Equal(t, 2, *events2[1].Event)

	require.Equal(t, 0, publisher.Len())

	events1 = puller1.Pull()
	require.Equal(t, 0, len(events1))

	events2 = puller2.Pull()
	require.Equal(t, 0, len(events2))
}

func TestEventAccum_MultiplePullersMultiplePulls(t *testing.T) {
	t.Parallel()

	publisher := updtree.NewEventsPullStorage[int]()
	puller1 := publisher.NewPuller()
	puller2 := publisher.NewPuller()
	publisher.Publish(1)
	publisher.Publish(2)

	require.Equal(t, 2, publisher.Len())

	events1 := puller1.Pull()
	require.Equal(t, 2, len(events1))
	require.Equal(t, 1, *events1[0].Event)
	require.Equal(t, 2, *events1[1].Event)

	require.Equal(t, 2, publisher.Len())

	publisher.Publish(3)
	publisher.Publish(4)

	require.Equal(t, 4, publisher.Len())

	events2 := puller2.Pull()
	require.Equal(t, 4, len(events2))
	require.Equal(t, 1, *events2[0].Event)
	require.Equal(t, 2, *events2[1].Event)
	require.Equal(t, 3, *events2[2].Event)
	require.Equal(t, 4, *events2[3].Event)

	require.Equal(t, 2, publisher.Len())

	events2 = puller2.Pull()
	require.Equal(t, 0, len(events2))

	events1 = puller1.Pull()
	require.Equal(t, 2, len(events1))
	require.Equal(t, 3, *events1[0].Event)
	require.Equal(t, 4, *events1[1].Event)

	require.Equal(t, 0, publisher.Len())

	publisher.Publish(3)
	publisher.Publish(4)

	require.Equal(t, 2, publisher.Len())
	lastEvt1 := puller1.Last()
	require.Equal(t, 4, *lastEvt1)

	require.Equal(t, 2, publisher.Len())
	lastEvt2 := puller2.Last()
	require.Equal(t, 4, *lastEvt2)

	require.Equal(t, 0, publisher.Len())

	nonExistantEvt := puller1.Last()
	require.Nil(t, nonExistantEvt)
}
