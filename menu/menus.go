package main

import (
    "fmt"
    "io/ioutil"
    "os"
    "os/exec"
    "strconv"
    "strings"
    "time"
)

//MenuItem holds an item for a menu, such as a button, a checkbox, or an input box
type MenuItem struct {
    Name   string `json:"name"`
    Type   string `json:"type"`   //menu, exec, explorer[:pwd], note, var name
    Action string `json:"action"` //var: string[:limit]|number[:min[:max]]|file[:extension1[,extension2,...]]|bool|opts:opt1,opt2,[opt3,...]
}

//MenuItemList holds a list of items to interact with
type MenuItemList struct {
    Title string            `json:"title"`
    Items []*MenuItem       `json:"items"` //items to display on the page
}

func (m *MenuItemList) AddItem(name, itemType, action string) {
    m.Items = append(m.Items, &MenuItem{Name: name, Type: itemType, Action: action})
}

//MenuEngine holds a list of menus and acts as the menu interface
type MenuEngine struct {
    //Menu navigation
    Menus       map[string]*MenuItemList
    HomeMenu    string
    LoadedMenu  string
    MenuHistory []string
    ItemHistory []int
    Environment map[string]string //global variables set by menus
    ItemCursor  int
    Locked      bool
    Return      string //return value set by some menu types

    //Rendering control
    Render func(string)
    LinesH int
    LinesV int
}

//NewMenuEngine returns a menu engine ready to be used
func NewMenuEngine(renderer func(string), width, height int) *MenuEngine {
    return &MenuEngine{
        Menus:       make(map[string]*MenuItemList),
        MenuHistory: make([]string, 0),
        ItemHistory: make([]int, 0),
        Environment: make(map[string]string),
        Render:      renderer,
        LinesH:      width,
        LinesV:      height,
    }
}

func (me *MenuEngine) LoadMenu(id string, itemList *MenuItemList) {
    me.Menus[id] = itemList
}

func (me *MenuEngine) Lock() {
    me.Locked = true
}
func (me *MenuEngine) Unlock() {
    me.Locked = false
}

func (me *MenuEngine) init() {
    if me.Menus == nil {
        me.Menus = make(map[string]*MenuItemList)
    }
    if me.MenuHistory == nil {
        me.MenuHistory = make([]string, 0)
    }
    if me.ItemHistory == nil {
        me.ItemHistory = make([]int, 0)
    }
    if me.Environment == nil {
    	me.Environment = make(map[string]string)
    }
}

func (me *MenuEngine) isBackVisible() bool {
    return len(me.MenuHistory) > 0
}

//PrevItem navigates to the previous menu item, or to the last if none previous
func (me *MenuEngine) PrevItem() {
    if me.Locked {
        return
    }
    me.init()
    defer me.render()

    if me.isBackVisible() && me.ItemCursor == -1 {
        me.ItemCursor = len(me.Menus[me.LoadedMenu].Items) - 1
    } else if !me.isBackVisible() && me.ItemCursor == 0 {
        me.ItemCursor = len(me.Menus[me.LoadedMenu].Items) - 1
    } else {
        me.ItemCursor--
    }

    if me.ItemCursor >= 0 && me.Menus[me.LoadedMenu].Items[me.ItemCursor].Type == "divider" {
        me.PrevItem()
    }
}

//NextItem navigates to the next menu item, or to the first if none next
func (me *MenuEngine) NextItem() {
    if me.Locked {
        return
    }
    me.init()
    defer me.render()

    if (me.ItemCursor + 1) >= len(me.Menus[me.LoadedMenu].Items) {
        if me.isBackVisible() {
            me.ItemCursor = -1
        } else {
            me.ItemCursor = 0
        }
    } else {
        me.ItemCursor++
    }

    if me.ItemCursor >= 0 && me.Menus[me.LoadedMenu].Items[me.ItemCursor].Type == "divider" {
        me.NextItem()
    }
}

//Action activates the selected item's action, such as navigating to a menu or executing a program
func (me *MenuEngine) Action() {
    if me.Locked {
        return
    }
    me.init()

    if me.ItemCursor == -1 {
        me.PrevMenu()
        return
    }

    selectedItem := me.Menus[me.LoadedMenu].Items[me.ItemCursor]
    selectedAction := me.Vars(selectedItem.Action)
    itemArgs := strings.Split(selectedItem.Type, " ")
    switch itemArgs[0] {
    case "internal":
        switch selectedAction {
        case "exit":
            os.Exit(0)
        default:
            me.ErrorText("Unknown internal action: " + selectedAction)
        }
    case "menu":
        me.ChangeMenu(selectedAction)
    case "exec":
        me.Lock()
		defer me.Unlock()
        cmdLine := strings.Split(selectedAction, " ")
        cmd := exec.Command(cmdLine[0])
        if len(cmdLine) > 1 {
            cmd = exec.Command(cmdLine[0], cmdLine[1:]...)
        }
        cmd.Stdout = os.Stdout
        cmd.Stdin = os.Stdin
        cmd.Stderr = os.Stderr
        err := cmd.Run()
        if err != nil {
        	fmt.Println(err)
	        os.Exit(0)
	    }
	    time.Sleep(3 * time.Second)
	    msg := "Task finished successfully!"
	    if len(itemArgs) > 1 {
	    	msg = strings.Join(itemArgs[1:], " ")
	    }
	   	me.ErrorText(msg)
    case "explorer":
        workingDir := "/"
        if len(itemArgs) > 1 {
            workingDir = strings.Join(itemArgs[1:], " ")
        }
        me.Explorer(workingDir, selectedAction)
    case "return":
    	if me.Return != "" {
	    	me.Environment[me.Return] = selectedAction
    		me.Return = ""
    	}
    	me.PrevMenu()
   	    
   	    //Back all the way out of an explorer context
   		for {
   			if string(me.Menus[me.LoadedMenu].Title[:8]) == "Explorer" {
	    		me.PrevMenu()
	    		continue
	    	}
	    	break
   		}
    case "setvar":
    	me.Return = itemArgs[1] //set var for what to return to

    	varAction := strings.Split(selectedAction, " ")
    	switch varAction[0] {
    	case "explorer":
    		workingDir := "/"
    		if len(varAction) > 1 {
    			workingDir = strings.Join(varAction[1:], " ")
    		}
    		me.Explorer(workingDir, "")
    	default:
    		me.ErrorText("Unknown action for var " + me.Return + ": " + selectedAction)
    	}
    case "note":
        if selectedAction != "" {
            me.ErrorText(selectedAction)
        } //Do nothing if it's just a note, show extended information if provided
    default:
        me.ErrorText("Unknown action: " + selectedItem.Type + ":" + selectedAction)
    }
}

//Explorer abuses the powers of AddMenu, ChangeMenu, and PrevMenu to create a file browser with support for passing a selected file to an executable
func (me *MenuEngine) Explorer(workingDir, bin string) {
    displayBin := workingDir
    if bin != "" {
        displayBin = strings.Replace(bin, "$?", workingDir, -1)
    }
    explorer := &MenuItemList{
        Title: "Explorer - " + displayBin,
        Items: make([]*MenuItem, 0),
    }

    dirStat, err := os.Stat(workingDir)
    if err != nil {
        switch {
            case os.IsNotExist(err):
                explorer.AddItem("Path " + workingDir + " does not exist!", "note", "")
            case os.IsExist(err):
                explorer.AddItem("Path " + workingDir + " is not accessible!", "note", "")
            default:
                explorer.AddItem("Path " + workingDir + " has unknown errors!", "note", fmt.Sprintf("%v", err))
        }
    } else {
        mode := dirStat.Mode()
        switch {
            case mode.IsDir(): //Directory
                files, err := ioutil.ReadDir(workingDir)
                if err != nil {
                    explorer.AddItem("Path " + workingDir + " has unreadable file contents!", "note", fmt.Sprintf("%v", err))
                } else {
                    for _, file := range files {
                        fileStat, err := os.Stat(workingDir + file.Name())
                        if err == nil {
                            switch {
                                case fileStat.IsDir():
                                    explorer.AddItem(file.Name() + "/", "explorer " + workingDir + file.Name() + "/", bin)
                                default:
                                	if bin != "" {
	                                    explorer.AddItem(file.Name(), "exec", strings.Replace(bin, "$?", fmt.Sprintf("%s%s", workingDir, file.Name()), -1))
	                                } else {
	                                	explorer.AddItem(file.Name(), "return", workingDir + file.Name())
	                                }
                            }
                        }
                    }
                }
            default: //File
                explorer.AddItem("You got pretty far into the switch nest!", "note", "")
        }
    }

    me.AddMenu(workingDir, explorer)
    me.ChangeMenu(workingDir)
}

//AddMenu adds a menu to the menu list
func (me *MenuEngine) AddMenu(menuID string, menu *MenuItemList) {
    me.init()
    me.Menus[menuID] = menu
}

//RemoveMenu removes a menu from the menu list
func (me *MenuEngine) RemoveMenu(menuID string) {
    me.init()
    me.Menus[menuID] = nil
}

//ChangeMenu changes to another available menu
func (me *MenuEngine) ChangeMenu(menuID string) {
    me.init()
    defer me.render()

    _, ok := me.Menus[menuID]
    if !ok {
        me.ErrorText("Unknown menu: " + menuID)
        return
    }

    if me.LoadedMenu != "" { //&& me.LoadedMenu != "INTERNAL_ERROR_TEXT" {
        me.MenuHistory = append(me.MenuHistory, me.LoadedMenu)
        me.ItemHistory = append(me.ItemHistory, me.ItemCursor)
    }

    me.LoadedMenu = menuID
    me.ItemCursor = 0
    if me.isBackVisible() {
        me.ItemCursor = -1
    }
}

//Home returns to the home menu
func (me *MenuEngine) Home() {
    me.ChangeMenu(me.HomeMenu)
}

//PrevMenu returns to the last menu in history
func (me *MenuEngine) PrevMenu() {
    me.init()
    defer me.render()

    if len(me.MenuHistory) == 0 {
        return //We can't go back to nothing, or can we?
    }

    menuID := me.MenuHistory[len(me.MenuHistory)-1]         //Get the previous menu
    me.MenuHistory = me.MenuHistory[:len(me.MenuHistory)-1] //Remove this menu from history regardless of it being valid
    itemCursor := me.ItemHistory[len(me.ItemHistory)-1]     //Get the previous item cursor
    me.ItemHistory = me.ItemHistory[:len(me.ItemHistory)-1] //Remove this item cursor from history regardless of it being valid

    _, ok := me.Menus[menuID]
    if !ok {
        //Allow returning to a working menu
        me.MenuHistory = append(me.MenuHistory, me.LoadedMenu)
        me.ItemHistory = append(me.ItemHistory, me.ItemCursor)

        me.ErrorText("Unknown menu: " + menuID)
        return
    }

    //Reset the item cursor if it's out of bounds
    if itemCursor >= len(me.Menus[menuID].Items) {
        me.ItemCursor = 0
    }

    me.LoadedMenu = menuID
    me.ItemCursor = itemCursor
}

//ErrorText generates an error message menu with menuID "INTERNAL_ERROR_TEXT" and navigates to it
//It is used internally as well as being made available, so refrain from using menuIDs starting with "INTERNAL"
func (me *MenuEngine) ErrorText(err string) {
    menuError := &MenuItemList{
        Title: err,
    }
    me.Menus["INTERNAL_ERROR_TEXT"] = menuError
    me.ChangeMenu("INTERNAL_ERROR_TEXT")
}

//GetRender returns a rendered menu text to be displayed immediately, as the menu state can change freely before and after
func (me *MenuEngine) GetRender() string {
    menu := ""

    lm := me.Menus[me.LoadedMenu]
    menu += "- " + lm.Title + "\n\n\n"
    if me.isBackVisible() {
        if me.ItemCursor == -1 {
            menu += "   --> Go back\n"
        } else {
            menu += "      Go back\n"
        }
	menu += "\n"
    }
    if len(lm.Items) > 0 {
        for i := 0; i < len(lm.Items); i++ {
            switch lm.Items[i].Type {
            case "divider":
                if length, err := strconv.Atoi(lm.Items[i].Action); err != nil {
                    for j := 0; j < length; j++ {
                        menu += "\n"
                    }
                } else {
                    menu += "\n"
                }
            default:
                if me.ItemCursor == i {
                    menu += "   --> " + lm.Items[i].Name + "\n"
                } else {
                    menu += "      " + lm.Items[i].Name + "\n"
                }
            }
        }
    }

    return me.Vars(menu)
}

//Vars returns a string formatted with all vars replaced
func (me *MenuEngine) Vars(in string) string {
	for varName, varValue := range me.Environment {
		in = strings.Replace(in, "$" + varName, varValue, -1)
	}
	return in
}

func (me *MenuEngine) render() {
    if me.Render != nil {
        me.Render(me.GetRender())
    }
}
