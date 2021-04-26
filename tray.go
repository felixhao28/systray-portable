package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"

	"github.com/getlantern/systray"
)

func main() {
	// Should be called at the very beginning of main().
	systray.Run(onReady, onExit)
}

func onExit() {
	os.Exit(0)
}

// Item represents an item in the menu
type Item struct {
	Icon       string `json:"icon"`
	Title      string `json:"title"`
	Tooltip    string `json:"tooltip"`
	Enabled    bool   `json:"enabled"`
	Checked    bool   `json:"checked"`
	Hidden     bool   `json:"hidden"`
	Items      []Item `json:"items"`
	InternalID int    `json:"__id"`
}

// Menu has an icon, title and list of items
type Menu struct {
	Icon    string `json:"icon"`
	Title   string `json:"title"`
	Tooltip string `json:"tooltip"`
	Items   []Item `json:"items"`
}

// Action for an item?..
type Action struct {
	Type  string `json:"type"`
	Item  Item   `json:"item"`
	Menu  Menu   `json:"menu"`
	SeqID int    `json:"seq_id"`
}

// ClickEvent for an click event
type ClickEvent struct {
	Type       string `json:"type"`
	Item       Item   `json:"item"`
	SeqID      int    `json:"seq_id"`
	InternalID int    `json:"__id"`
}

func readJSON(reader *bufio.Reader, v interface{}) error {
	input, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	if len(input) < 1 {
		return fmt.Errorf("Empty line")
	}

	// fmt.Fprintf(os.Stderr, "got line: %s\n", input)

	lineReader := strings.NewReader(input[0 : len(input)-1])
	if err := json.NewDecoder(lineReader).Decode(v); err != nil {
		return err
	}

	return nil
}

func addMenuItem(items *[]*systray.MenuItem, rawItems *[]*Item, seqID2InternalID *[]int, internalID2SeqID *map[int]int, item *Item, parent *systray.MenuItem) {
	if item.Title == "<SEPARATOR>" {
		systray.AddSeparator()
		*rawItems = append(*rawItems, item)
		*items = append(*items, nil)
	} else {
		var menuItem *systray.MenuItem
		if parent == nil {
			menuItem = systray.AddMenuItem(item.Title, item.Tooltip)
		} else {
			menuItem = parent.AddSubMenuItem(item.Title, item.Tooltip)
		}
		if item.Checked {
			menuItem.Check()
		} else {
			menuItem.Uncheck()
		}
		if item.Enabled {
			menuItem.Enable()
		} else {
			menuItem.Disable()
		}
		if len(item.Icon) > 0 {
			icon, err := base64.StdEncoding.DecodeString(item.Icon)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				menuItem.SetIcon(icon)
			}
		}
		for i := 0; i < len(item.Items); i++ {
			subitem := item.Items[i]
			addMenuItem(items, rawItems, seqID2InternalID, internalID2SeqID, &subitem, menuItem)
		}
		*rawItems = append(*rawItems, item)
		*items = append(*items, menuItem)
	}
	seqID := len(*items) - 1
	(*internalID2SeqID)[item.InternalID] = seqID
	*seqID2InternalID = append(*seqID2InternalID, item.InternalID)
}

func onReady() {
	signalChannel := make(chan os.Signal, 2)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		for sig := range signalChannel {
			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				//handle SIGINT, SIGTERM
				fmt.Fprintln(os.Stderr, "Quit")
				systray.Quit()
			default:
				fmt.Fprintln(os.Stderr, "Unhandled signal:", sig)
			}
		}
	}()

	// We can manipulate the systray in other goroutines
	go func() {
		rawItems := make([]*Item, 0)
		items := make([]*systray.MenuItem, 0)
		seqID2InternalID := make([]int, 0)
		internalID2SeqID := make(map[int]int)
		fmt.Println(`{"type": "ready"}`)
		reader := bufio.NewReader(os.Stdin)

		var menu Menu
		if err := readJSON(reader, &menu); err != nil {
			fmt.Fprintln(os.Stderr, err)
			systray.Quit()
			return
		}
		// fmt.Println("menu", menu)

		icon, err := base64.StdEncoding.DecodeString(menu.Icon)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			systray.Quit()
			return
		}

		systray.SetIcon(icon)
		systray.SetTitle(menu.Title)
		systray.SetTooltip(menu.Tooltip)

		updateItem := func(action Action) {
			item := action.Item
			var seqID int
			if action.SeqID < 0 {
				seqID = internalID2SeqID[action.Item.InternalID]
			} else {
				seqID = action.SeqID
			}
			menuItem := items[seqID]
			rawItems[seqID] = &item
			if menuItem == nil {
				return
			}
			if item.Hidden {
				menuItem.Hide()
			} else {
				if item.Checked {
					menuItem.Check()
				} else {
					menuItem.Uncheck()
				}
				if item.Enabled {
					menuItem.Enable()
				} else {
					menuItem.Disable()
				}
				menuItem.SetTitle(item.Title)
				menuItem.SetTooltip(item.Tooltip)
				if len(item.Icon) > 0 {
					icon, err := base64.StdEncoding.DecodeString(item.Icon)
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
					}
					menuItem.SetIcon(icon)
				}
				menuItem.Show()
				for _, child := range item.Items {
					seqID = internalID2SeqID[child.InternalID]
					items[seqID].Show()
				}
			}
			// fmt.Println("Done")
			// fmt.Printf("Read from channel %#v and received %s\n", items[chosen], value.String())
		}
		updateMenu := func(action Action) {
			m := action.Menu
			if menu.Title != m.Title {
				menu.Title = m.Title
				systray.SetTitle(menu.Title)
			}
			if menu.Icon != m.Icon && m.Icon != "" {
				menu.Icon = m.Icon
				icon, err := base64.StdEncoding.DecodeString(menu.Icon)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
				} else {
					systray.SetIcon(icon)
				}
			}
			if menu.Tooltip != m.Tooltip {
				menu.Tooltip = m.Tooltip
				systray.SetTooltip(menu.Tooltip)
			}
		}

		update := func(action Action) {
			switch action.Type {
			case "update-item":
				updateItem(action)
			case "update-menu":
				updateMenu(action)
			case "update-item-and-menu":
				updateItem(action)
				updateMenu(action)
			case "exit":
				systray.Quit()
			}
		}

		for i := 0; i < len(menu.Items); i++ {
			item := menu.Items[i]
			addMenuItem(&items, &rawItems, &seqID2InternalID, &internalID2SeqID, &item, nil)
		}

		go func(reader *bufio.Reader) {
			for i := 0; i < len(menu.Items); i++ {
				item := menu.Items[i]
				menuItem := items[i]
				if menuItem != nil && item.Hidden {
					menuItem.Hide()
				}
			}
			for {
				var action Action
				if err := readJSON(reader, &action); err != nil {
					fmt.Fprintln(os.Stderr, err)
					systray.Quit()
					break
				}
				update(action)
			}
		}(reader)

		stdoutEnc := json.NewEncoder(os.Stdout)
		// {"type": "update-item", "item": {"title":"aa3","tooltip":"bb","enabled":true,"checked":true}, "__id": 0}
		for {
			itemsCnt := 0
			for _, ch := range items {
				if ch != nil {
					itemsCnt++
				}
			}
			cases := make([]reflect.SelectCase, itemsCnt)
			caseCnt2SeqID := make([]int, len(items))
			itemsCnt = 0
			for i, ch := range items {
				if ch == nil {
					continue
				}
				cases[itemsCnt] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch.ClickedCh)}
				caseCnt2SeqID[itemsCnt] = i
				itemsCnt++
			}

			remaining := len(cases)
			for remaining > 0 {
				chosen, _, ok := reflect.Select(cases)
				if !ok {
					// The chosen channel has been closed, so zero out the channel to disable the case
					cases[chosen].Chan = reflect.ValueOf(nil)
					remaining--
					continue
				}
				seqID := caseCnt2SeqID[chosen]
				// menuItem := items[chosen]
				err := stdoutEnc.Encode(ClickEvent{
					Type:       "clicked",
					Item:       *rawItems[seqID],
					SeqID:      seqID,
					InternalID: seqID2InternalID[seqID],
				})
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			}
		}
	}()
}
