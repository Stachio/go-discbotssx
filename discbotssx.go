package discbotssx

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Stachio/go-printssx"
	"github.com/bwmarrin/discordgo"
)

// Printer - Generic printer object provided by stachio/printerssx
var Printer = printssx.New("BOT", log.Println, log.Printf, printssx.Loud, printssx.Loud)

type Result int

const Success Result = 0
const Warning Result = 1
const Error Result = 3
const Fatal Result = 4
const Exit Result = 5

type Output struct {
	result  Result
	details string
}

func NewOutput(result Result, details string) (output *Output) {
	return &Output{result: result, details: details}
}

// Bot - Discord bot object
type Bot struct {
	owner      string
	session    *discordgo.Session
	commandMap map[string]Command
	alive      bool
}

type Bundle struct {
	bot     *Bot
	Session *discordgo.Session
	Message *discordgo.MessageCreate
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
	var noise printssx.Noise
	var status string
	switch result {
	case Success:
		noise = printssx.Loud
		status = "SUCCESS"
	case Warning:
		noise = printssx.Moderate
		status = "WARNING"
	case Error:
		noise = printssx.Subtle
		status = "ERROR"
		// Send an error report to the owner
		channel, err := bundle.Session.UserChannelCreate(bundle.bot.owner)
		bundle.Session.ChannelMessageSend(channel.ID, fmt.Sprintf("Figure this error out bud \"%s\"", err.Error()))
	case Fatal:
		noise = printssx.Subtle
		status = "FATAL"
	case Exit:
		noise = printssx.Subtle
		status = "EXIT"
		bundle.bot.alive = false
	}
	Printer.Printf(noise, "FINISH cmd:%s args:%v result:%s", args[0], args[1:], status)
}

func (bundle *Bundle) RunCommand(line string) {
	args := strings.Split(line, " ")
	cmd, ok := bundle.bot.commandMap[args[0]]
	if ok {
		Printer.Printf(printssx.Subtle, "START cmd:%s args %v\n", args[0], args[1:])
		result, err := cmd(bundle, args)
		bundle.result(args, result, err)
		//outputs := bundle.commandPump(cmd, args[1:])
		//for _, output := range outputs {
		//output.Process(bot)
		//}
	}
}

func (bot *Bot) messageHandler(bwSession *discordgo.Session, bwMessage *discordgo.MessageCreate) {
	bundle := &Bundle{bot: bot, Session: bwSession, Message: bwMessage}
	bundle.RunCommand(bwMessage.Message.Content)
}

// New - Initializes the bot type with a discord session bound to "token"
func New(token, owner string) (bot *Bot, err error) {
	bot = &Bot{owner: owner, alive: false, commandMap: make(map[string]Command)}

	session, err := discordgo.New(fmt.Sprintf("Bot %s", token))
	if err != nil {
		return nil, err
	}

	session.AddHandler(bot.messageHandler)
	bot.session = session
	Printer.Println(printssx.Moderate, "Discord bot created")
	return
}

// AddCommand - Adds a Command object to the bot's commandMap
func (bot *Bot) AddCommand(cmdStr string, cmd Command) {
	bot.commandMap[cmdStr] = cmd
}

func (bot *Bot) Run() (err error) {
	Printer.Println(printssx.Subtle, "Running discord bot...")
	bot.alive = true
	err = bot.session.Open()

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
	_, err = bot.session.ChannelMessageSend(channel.ID, "Bot stopped")
	bot.session.Close()
	return
}
