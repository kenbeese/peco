package peco

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nsf/termbox-go"
	"github.com/peco/peco/keyseq"
)

// Keymap holds all the key sequence to action map
type Keymap struct {
	Config map[string]string
	Action map[string][]string // custom actions
	Keyseq *keyseq.Keyseq
}

// NewKeymap creates a new Keymap struct
func NewKeymap(config map[string]string, actions map[string][]string) Keymap {
	return Keymap{config, actions, keyseq.New()}

}

// Handler returns the appropriate action for the given termbox event
func (km Keymap) Handler(ev termbox.Event) Action {
	modifier := keyseq.ModNone
	if (ev.Mod & termbox.ModAlt) != 0 {
		modifier = keyseq.ModAlt
	}

	key := keyseq.Key{modifier, ev.Key, ev.Ch}
	action, err := km.Keyseq.AcceptKey(key)

	switch err {
	case nil:
		// Found an action!
		return wrapClearSequence(action.(Action))
	case keyseq.ErrInSequence:
		return wrapRememberSequence(ActionFunc(doNothing))
	default:
		return wrapClearSequence(ActionFunc(doAcceptChar))
	}
}

func wrapRememberSequence(a Action) Action {
	return ActionFunc(func(i *Input, ev termbox.Event) {
		s, err := keyseq.EventToString(ev)
		if err == nil {
			i.currentKeySeq = append(i.currentKeySeq, s)
			i.SendStatusMsg(strings.Join(i.currentKeySeq, " "))
		}
		a.Execute(i, ev)
	})
}

func wrapClearSequence(a Action) Action {
	return ActionFunc(func(i *Input, ev termbox.Event) {
		s, err := keyseq.EventToString(ev)
		if err == nil {
			i.currentKeySeq = append(i.currentKeySeq, s)
		}

		if len(i.currentKeySeq) > 0 {
			i.SendStatusMsgAndClear(strings.Join(i.currentKeySeq, " "), 500*time.Millisecond)
			i.currentKeySeq = []string{}
		}

		a.Execute(i, ev)
	})
}

const maxResolveActionDepth = 100

func (km Keymap) resolveActionName(name string, depth int) (Action, error) {
	if depth >= maxResolveActionDepth {
		return nil, fmt.Errorf("error: Could not resolve %s: deep recursion", name)
	}

	// Can it be resolved via regular nameToActions ?
	v, ok := nameToActions[name]
	if ok {
		return v, nil
	}

	// If all else fails...
	// Finish is a special case. We can dynamically create  a finish
	// function that exits with an arbitrary exit status
	if strings.HasPrefix(name, "peco.Finish") {
		intstr, err := strconv.ParseInt(name[11:], 10, 64)
		if err == nil {
			v = makeFinishAction(int(intstr))
			nameToActions[name] = v
			return v, nil
		}
	}

	// Can it be resolved via combined actions?
	l, ok := km.Action[name]
	if ok {
		actions := []Action{}
		for _, actionName := range l {
			child, err := km.resolveActionName(actionName, depth+1)
			if err != nil {
				return nil, err
			}
			actions = append(actions, child)
		}
		v = makeCombinedAction(actions...)
		nameToActions[name] = v
		return v, nil
	}

	return nil, fmt.Errorf("error: Could not resolve %s: no such action", name)
}

// ApplyKeybinding applies all of the custom key bindings on top of
// the default key bindings
func (km Keymap) ApplyKeybinding() {
	k := km.Keyseq
	k.Clear()

	// Copy the map
	kb := map[string]Action{}
	for s, a := range defaultKeyBinding {
		kb[s] = a
	}

	// munge the map using config
	for s, as := range km.Config {
		if as == "-" {
			delete(kb, s)
			continue
		}

		v, err := km.resolveActionName(as, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		kb[s] = v
	}

	// now compile using kb
	for s, a := range kb {
		list, err := keyseq.ToKeyList(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unknown key %s: %s", s, err)
			continue
		}

		k.Add(list, a)
	}

	k.Compile()
}

// TODO: this needs to be fixed.
func (km Keymap) hasModifierMaps() bool {
	return false
}
