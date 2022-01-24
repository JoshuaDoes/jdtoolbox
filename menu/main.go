package main

import (
	"fmt"
	"io/ioutil"
	"os"
//	"os/exec"
	"os/signal"
	"path/filepath"
//	"strconv"
//	"strings"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/JoshuaDoes/json"
)

type MenuConfig struct {
	Environment map[string]string  `json:"environment"`
	HomeMenu string`json:"homeMenu"`
	Menus map[string]*MenuItemList `json:"menus"`
	Keyboards map[string][]*MenuKeycodeBinding `json:"keyboards"`
}
type MenuKeycodeBinding struct {
	Keycode   uint16 `json:"keycode"`
	Action string `json:"action"`
	OnRelease bool   `json:"onRelease"`
}

var (
	configFile string //path to menu configuration
	keyCalibrationFile string //path to keyboard calibration, can be written for embedded devices or generated by first run calibrator
	hLines int//horizontal lines for screen
	vLines int//vertical lines for screen
	workingDir string //working directory for menu assets

	keyCalibration map[string][]*MenuKeycodeBinding = make(map[string][]*MenuKeycodeBinding)
	menuConfig *MenuConfig //menu configuration
	menuEngine *MenuEngine //menu engine/runtime/???
)

func init() {
	//Apply all command-line flags
	flag.StringVar(&configFile, "menu", "/etc/jdtoolbox/menu.json", "path to menu configuration")
	flag.StringVar(&keyCalibrationFile, "keyCalibration", "/etc/jdtoolbox/keyCalibration.json", "path to keyboard calibration, generated by calibrator if not present")
	flag.IntVar(&hLines, "hLines", 0, "horizontal lines available to virtual screen") //<= 0: unlimited
	flag.IntVar(&vLines, "vLines", 0, "vertical lines available to virtual screen") //<= 0: unlimited
	flag.StringVar(&workingDir, "workingDir", "/", "the root directory of menu assets")
	flag.Parse()

	if vLines > 0 {
		vLines += 15
	}

	keyCalibrationJSON, err := ioutil.ReadFile(keyCalibrationFile)
	if err == nil {
		keyCalibration = make(map[string][]*MenuKeycodeBinding)
		err = json.Unmarshal(keyCalibrationJSON, &keyCalibration)
		if err != nil {
			panic(fmt.Sprintf("error parsing key calibration file: %v", err))
		}
	}

	configJSON, err := ioutil.ReadFile(configFile)
	if err != nil {
		panic(fmt.Sprintf("error reading config file: %v", err))
	}

	menuConfig = &MenuConfig{}
	err = json.Unmarshal(configJSON, menuConfig)
	if err != nil {
		panic(fmt.Sprintf("error parsing config file: %v", err))
	}

	menuEngine = NewMenuEngine(render, hLines, vLines)
	menuEngine.Environment["WORKINGDIR"] = workingDir

	for id, itemList := range menuConfig.Menus {
		menuEngine.AddMenu(id, itemList)
	}

	menuEngine.HomeMenu = menuConfig.HomeMenu

	//DEPRECATED, move embedded keyboards to key calibrator
	if menuConfig.Keyboards != nil && len(menuConfig.Keyboards) > 0 {
		for keyboard, bindings := range menuConfig.Keyboards {
			kl, err := NewKeycodeListener(keyboard)
			if err != nil {
				panic(fmt.Sprintf("error listening to keyboard %s: %v", keyboard, err))
			}
			for _, binding := range bindings {
				var action func()
				switch binding.Action {
					case "prevItem":
						action = menuEngine.PrevItem
					case "nextItem":
						action = menuEngine.NextItem
					case "selectItem":
						action = menuEngine.Action
					default:
						panic("unknown action: " + binding.Action)
				}
				kl.Bind(binding.Keycode, binding.OnRelease, action)
			}
			go kl.Run()
		}
	}

	if keyCalibration != nil && len(keyCalibration) > 0 {
		bindKeys()
	}
}

func bindKeys() {
	for keyboard, bindings := range keyCalibration {
		kl, err := NewKeycodeListener(keyboard)
		if err != nil {
			panic(fmt.Sprintf("error listening to keyboard %s: %v", keyboard, err))
		}
		for _, binding := range bindings {
			var action func()
			switch binding.Action {
				case "prevItem":
					action = menuEngine.PrevItem
				case "nextItem":
					action = menuEngine.NextItem
				case "selectItem":
					action = menuEngine.Action
				default:
					panic("unknown action: " + binding.Action)
			}
			kl.Bind(binding.Keycode, binding.OnRelease, action)
		}
		go kl.Run()
	}
}

type KeyCalibration struct {
	Ready bool
	Action string
}
func (kc *KeyCalibration) Input(keyboard string, keycode uint16, onRelease bool) {
	if !kc.Ready {
		os.Exit(0)
	}
	if kc.Action == "" {
		return
	}
	if onRelease {
		return
	}
	if keyCalibration[keyboard] == nil {
		keyCalibration[keyboard] = make([]*MenuKeycodeBinding, 0)
	}
	keyCalibration[keyboard] = append(keyCalibration[keyboard], &MenuKeycodeBinding{
		Keycode: keycode,
		Action: kc.Action,
	})
	kc.Action = ""
}

func main() {
	//Generate a key calibration file if one doesn't exist yet
	if _, err := os.Stat(keyCalibrationFile); err != nil {
		calibrator := &KeyCalibration{}

		//Get a list of keyboards
		keyboards := make([]string, 0)
		err := filepath.Walk("/dev/input", func(path string, info os.FileInfo, err error) error {
			if len(path) < 16 || string(path[:16]) != "/dev/input/event" {
				return nil
			}
			keyboards = append(keyboards, path)
			return nil
		})
		if err != nil {
			panic(fmt.Sprintf("error walking inputs: %v", err))
		}

		//Bind all keyboards to calibrator input
		for _, keyboard := range keyboards {
			kl, err := NewKeycodeListener(keyboard)
			if err != nil {
				panic(fmt.Sprintf("error listening to walked keyboard %s: %v", keyboard, err))
			}
			kl.RootBind = calibrator.Input
			go kl.Run()
		}

		//Start calibrating!
		stages := 5
		for stage := 0; stage < stages; stage++ {
			switch stage {
				case 0:
					clear(4)
					fmt.Println("Welcome to the keyboard calibrator!")
					fmt.Println("Press any key in the next 3 seconds to cancel, or wait to continue.")
					fmt.Println("")
					fmt.Println("")
					fmt.Println("")
					time.Sleep(time.Second * 3)
					calibrator.Ready = true
				case 1:
					clear(2)
					calibrator.Action = "selectItem"
					fmt.Println("Press any key to use to select a menu item.")
					fmt.Println("If you have a touch screen or a fingerprint sensor, tap it!")
					fmt.Println("")
					fmt.Println("")
					fmt.Println("")
					for calibrator.Action != "" {}
				case 2:
					clear(2)
					calibrator.Action = "prevItem"
					fmt.Println("Press any key to use to navigate up in a menu.")
					fmt.Println("")
					fmt.Println("")
					fmt.Println("")
					for calibrator.Action != "" {}
				case 3:
					clear(2)
					calibrator.Action = "nextItem"
					fmt.Println("Press any key to use to navigate down in a menu.")
					fmt.Println("")
					fmt.Println("")
					fmt.Println("")
					for calibrator.Action != "" {}
				case 4:
					clear(2)
					fmt.Println("Calibration complete!")
					fmt.Println("Saving calibration results...")
					keyboards, err := json.Marshal(keyCalibration, true)
					if err != nil {
						panic(fmt.Sprintf("error encoding calibration results: %v", err))
					}
					keyboardsFile, err := os.Create(keyCalibrationFile)
					if err != nil {
						panic(fmt.Sprintf("error creating calibration file: %v", err))
					}
					defer keyboardsFile.Close()
					_, err = keyboardsFile.Write(keyboards)
					if err != nil {
						panic(fmt.Sprintf("error writing calibration file: %v", err))
					}
					fmt.Println("")
					fmt.Println("")
					fmt.Println("")
					fmt.Println("Saved results:", keyCalibrationFile)
					//fmt.Println(string(keyboards))
					bindKeys()
					time.Sleep(time.Second * 2)
					//calibrator.Ready = false
			}
		}
	}

	clear(4)
	menuEngine.Home()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT)
	<-sc
}

func render(menu string) {
	clear(0)
	fmt.Printf("  " + menu)
	fmt.Printf("\n\n\n")
}

func clear(delay time.Duration) {
	for lines := 0; lines < menuEngine.LinesV; lines++ {
		fmt.Printf("\n")
		if delay > 0 {
			time.Sleep(delay * time.Millisecond)
		}
	}
}