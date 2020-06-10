package discbotssx

import (
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	logger "github.com/Stachio/go-loggerssx"

	"github.com/Stachio/go-printssx"
	"github.com/bwmarrin/discordgo"
)

// Printer - Generic printer object provided by stachio/printerssx
var Printer = printssx.New("BOT", log.Println, log.Printf, printssx.Loud, printssx.Loud)

type Result int

const Success Result = 0
const Warning Result = 1

//const Error Result = 3
const Fatal Result = 3
const Exit Result = 4

type Output struct {
	result  Result
	details string
}

func NewOutput(result Result, details string) (output *Output) {
	return &Output{result: result, details: details}
}

// Config - Discord config object
type Config struct {
	Token   []byte `xml:"token"`
	Owner   string `xml:"owner"`
	LogPath string `xml:"logpath"`
}

func NewConfig(configPath string) (*Config, error) {
	config := &Config{}
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	err = xml.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// Bot - Discord bot object
type Bot struct {
	logPath      string
	owner        string
	id           string
	session      *discordgo.Session
	commandMap   map[string]Command
	inlineMap    map[string]Command
	customMap    map[string]Command
	cancelations []Cancel
	alive        bool
}

func (bot *Bot) Session() *discordgo.Session {
	return bot.session
}

func (bot *Bot) Alive() bool {
	return bot.alive
}

type Bundle struct {
	bot      *Bot
	Log      *log.Logger
	session  *discordgo.Session
	message  *discordgo.MessageCreate
	cmdIndex int
}

func (bundle *Bundle) Session() *discordgo.Session {
	return bundle.session
}

func (bundle *Bundle) Message() *discordgo.MessageCreate {
	return bundle.message
}

func (bundle *Bundle) Owner() string {
	return bundle.bot.owner
}

func (bundle *Bundle) CmdIndex() int {
	return bundle.cmdIndex
}

// Error - Package defined error struct to house sql statements
type Error struct {
	operation string
	goerr     error
}

func (err *Error) Error() string {
	return "Operation: " + err.operation + "\nError: " + err.goerr.Error()
}

// NewError - returns custom error type
func NewError(operation string, err error) *Error {
	return &Error{operation: operation, goerr: err}
}

// Command - Generic type to shorten func declaration
type Command func(*Bundle, []string) (Result, error)

/*
func (bundle *Bundle) commandPump(cmd []Command, args []string) (out []Output) {
	var outputs []Output
	for _, funcCall := range cmd {
		args, outputs = funcCall(bundle, args)
		for _, output := range outputs {
			out = append(out, output)
		}
	}
	return
}
*/

//
func (bundle *Bundle) result(args []string, result Result, err error) {
	//var noise printssx.Noise
	var status string
	switch result {
	case Success:
		//noise = printssx.Loud
		status = "SUCCESS"
	case Warning:
		//noise = printssx.Moderate
		status = "WARNING"
	case Fatal:
		//noise = printssx.Subtle
		status = "FATAL"
	case Exit:
		//noise = printssx.Subtle
		status = "EXIT"
		bundle.bot.alive = false
	}

	if err != nil {
		//noise = printssx.Subtle
		status = "ERROR"
		// Send an error report to the owner
		// Don't use err in the return ya idgit
		channel, erro := bundle.session.UserChannelCreate(bundle.bot.owner)
		if erro != nil {
			panic(erro)
		}
		_, erro = bundle.session.ChannelMessageSend(channel.ID, fmt.Sprintf("Figure this error out bud \"%s\"", err.Error()))
		if erro != nil {
			panic(erro)
		}
		_, erro = bundle.session.ChannelMessageSend(bundle.message.ChannelID, (bundle.message.Author.Mention() + " that didn't go as planned"))
		if erro != nil {
			panic(erro)
		}
	}

	bundle.Log.Printf("FINISH cmd:%s line:%v result:%s", args[bundle.cmdIndex], args, status)
}

func (bundle *Bundle) RunCustoms(args []string) {
	for name, cmd := range bundle.bot.customMap {
		bundle.Log.Printf("START custom cmd:%s args %v\n", name, args)
		result, err := cmd(bundle, args)
		bundle.result(args, result, err)
	}
}

func (bundle *Bundle) RunInlines(args []string) {
	var indie int
	var arg string
	var cmd Command
	var ok bool

	for indie, arg = range args {
		if cmd, ok = bundle.bot.inlineMap[arg]; ok {
			bundle.cmdIndex = indie
			break
		}
	}

	if ok {
		bundle.Log.Printf("START inline cmd:%s line %v\n", args[indie], args)
		result, err := cmd(bundle, args)
		bundle.result(args, result, err)
	}
}

func (bundle *Bundle) RunCommand(args []string) {
	if len(args) == 0 {
		bundle.Log.Print("START no cmd available")
		return
	}

	cmd, ok := bundle.bot.commandMap[args[0]]
	if ok {
		bundle.Log.Printf("START cmd:%s args %v\n", args[0], args[1:])
		result, err := cmd(bundle, args)
		bundle.result(args, result, err)
		//outputs := bundle.commandPump(cmd, args[1:])
		//for _, output := range outputs {
		//output.Process(bot)
		//}
	}
}

type Cancel func(*Bundle) bool

func (bot *Bot) AddCancelation(cancel Cancel) {
	bot.cancelations = append(bot.cancelations, cancel)
}

func (bot *Bot) messageHandler(bwSession *discordgo.Session, bwMessage *discordgo.MessageCreate) {
	//guildID := bwMessage.GuildID
	authorID := bwMessage.Author.ID
	//channelID := bwMessage.ChannelID
	messageID := bwMessage.Message.ID

	if authorID == bot.id {
		// Message originated from current bot
		return
	}

	uniqueID := time.Now().Format("2006-01-02T15-04-05") + " " + messageID //strings.Join([]string{channelID, messageID}, " ")
	args := strings.Split(bwMessage.Message.Content, " ")

	newArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if len(arg) > 0 {
			newArgs = append(newArgs, arg)
		}
	}

	Printer.Println(printssx.Subtle, "[START] message ID", uniqueID, newArgs)
	fileName := uniqueID + ".log"
	logPath := filepath.Join(bot.logPath, fileName)
	fileLogger, err := logger.New(logPath, false)
	fmt.Println(logPath)
	if err != nil {
		panic(err)
	}
	logger := log.New(fileLogger, "", log.Flags())

	bundle := &Bundle{
		bot:     bot,
		Log:     logger,
		session: bwSession,
		message: bwMessage,
	}
	for _, cancel := range bot.cancelations {
		if cancel(bundle) {
			Printer.Println(printssx.Subtle, "[CANCEL] message ID", uniqueID)
			return
		}
	}
	bundle.RunCustoms(newArgs)
	bundle.RunInlines(newArgs)
	bundle.RunCommand(newArgs)
	Printer.Println(printssx.Subtle, "[FINISH] message ID", uniqueID)
}

// New - Initializes the bot type with a discord session bound to "token"
// Token is automatically randomized
func New(token []byte, owner, logPath string) (bot *Bot, err error) {
	bot = &Bot{owner: owner, logPath: logPath, alive: false,
		commandMap: make(map[string]Command),
		inlineMap:  make(map[string]Command),
		customMap:  make(map[string]Command),
	}

	session, err := discordgo.New(fmt.Sprintf("Bot %s", string(token)))
	rand.Read(token)
	if err != nil {
		return nil, err
	}

	session.AddHandler(bot.messageHandler)
	// Increased session client timeout from 20 to 60 seconds
	session.Client = &http.Client{Timeout: (60 * time.Second)}
	bot.session = session
	me, err := session.User("@me")
	if err != nil {
		return nil, err
	}
	bot.id = me.ID
	Printer.Println(printssx.Moderate, "Discord bot created")
	return
}

// NewWithConfig - Initializes the bot type with the config object (token, owner, logpath)
func NewWithConfig(config *Config) (bot *Bot, err error) {
	return New(config.Token, config.Owner, config.LogPath)
}

// NewWithConfigFile - Initialized the bot object with a config file
func NewWithConfigFile(configFile string) (bot *Bot, err error) {
	config, err := NewConfig(configFile)
	if err != nil {
		return nil, err
	}
	return NewWithConfig(config)
}

// AddCommand - Adds a Command object to the bot's commandMap
func (bot *Bot) AddCommand(cmdStr string, cmd Command) {
	bot.commandMap[cmdStr] = cmd
}

func (bot *Bot) AddInline(cmdStr string, cmd Command) {
	bot.inlineMap[cmdStr] = cmd
}

func (bot *Bot) AddCustom(name string, cmd Command) {
	bot.customMap[name] = cmd
}

func (bot *Bot) Run() (err error) {
	Printer.Println(printssx.Subtle, "Running discord bot...")
	err = bot.session.Open()
	bot.alive = true

	channel, err := bot.session.UserChannelCreate(bot.owner)
	if err != nil {
		return
	}
	_, err = bot.session.ChannelMessageSend(channel.ID, "Bot started")
	if err != nil {
		return
	}
	Printer.Println(printssx.Subtle, "Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	Printer.Println(printssx.Subtle, "Shutting down discord bot...")
	channel, err = bot.session.UserChannelCreate(bot.owner)
	_, err = bot.session.ChannelMessageSend(channel.ID, "Bot stopped")
	bot.session.Close()
	return
}
