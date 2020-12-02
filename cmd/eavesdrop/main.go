// Eavesdrop is a CLI application that joins a voice channel and plays
// everything in it.
package main

import (
	"log"
	"os"
	"time"

	"github.com/diamondburned/arikawa/v2/discord"
	"github.com/diamondburned/arikawa/v2/state"
	"github.com/diamondburned/arikawa/v2/voice"
	"github.com/diamondburned/vocalcord/player"
	"github.com/diamondburned/vocalcord/player/otopus"
	"github.com/pkg/errors"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalln("Invalid arguments; usage: eavesdrop channelID")
	}

	id, err := discord.ParseSnowflake(os.Args[1])
	if err != nil {
		log.Fatalln("Failed to parse snowflake:", err)
	}

	token := os.Getenv("TOKEN")
	if token == "" {
		log.Fatalln("Missing $TOKEN.")
	}

	if err := eavesdrop(token, discord.ChannelID(id)); err != nil {
		log.Fatalln("Failed to eavesdrop:", err)
	}
}

func eavesdrop(token string, channelID discord.ChannelID) error {
	state, err := state.New(token)
	if err != nil {
		return errors.Wrap(err, "failed to make state")
	}

	if err := state.Open(); err != nil {
		return errors.Wrap(err, "failed to open gateway")
	}

	defer state.Close()

	ch, err := state.Channel(channelID)
	if err != nil {
		return errors.Wrap(err, "failed to get channel")
	}

	pool, err := otopus.NewPool(1, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to make an otopus pool")
	}
	defer pool.Close()

	v, err := voice.NewSession(state)
	if err != nil {
		return errors.Wrap(err, "failed to create new session")
	}

	pl := player.NewPlayer(v, pool)

	if err := v.JoinChannel(ch.GuildID, ch.ID, true, false); err != nil {
		return errors.Wrap(err, "failed to join channel")
	}
	defer v.Leave()

	if err := pl.Start(); err != nil {
		return errors.Wrap(err, "failed to start player")
	}

	return nil
}
