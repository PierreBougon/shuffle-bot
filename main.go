package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// cache tables
var guildIDs map[string]string
var usernames map[string]discordgo.User

var supportedGames = []string {"Valorant"}
var valorantMaps = []string {"Bind", "Split", "Haven"}
var teamsNames = []string {"maoune", "spp"}
var sides = []string {"CT", "T"}

func main() {
	rand.Seed(time.Now().UnixNano())
	dg, err := discordgo.New("Bot " + "TOKEN")
	if err != nil {
		fmt.Println("Error creating Discord bot: ", err)
		return
	}

	guildIDs = make(map[string]string)
	usernames = make(map[string]discordgo.User)
	dg.AddHandler(messageHandler)
	dg.AddHandler(userPresenceUpdateHandler)

	dg.Open()
	if err != nil {
		fmt.Println("Error opening WebSocket connection: ", err)
		return
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func sendReply(s *discordgo.Session, m *discordgo.MessageCreate, str string) {
	sendMessage := fmt.Sprintf("<@!%s> ", m.Author.ID)
	sendMessage += str
	s.ChannelMessageSend(m.ChannelID, sendMessage)
}

func isContain(needle string, haystack []string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func userPresenceUpdateHandler(s *discordgo.Session, p *discordgo.PresenceUpdate) {
	if p.User.Username != "" {
		// update cache
		fmt.Println("Username changed: " + p.User.Username)
		usernames[p.User.ID] = *p.User
	}
}

func messageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	// translate channelID -> guildID to reduce latency
	// This does not need use in case of building with latest discordgo's develop branch
	gid, ok := guildIDs[m.ChannelID]
	if !ok {
		fmt.Println("Cache MISS")
		// cache miss
		sourceTextChannel, err := s.Channel(m.ChannelID)
		if err != nil {
			fmt.Println("Error while fetching source channel: ", err)
			return
		}
		gid = sourceTextChannel.GuildID
		guildIDs[m.ChannelID] = gid
	}

	if gid == "" {
		// Invoked from user chat directly
		s.ChannelMessageSend(m.ChannelID, "Please send after connecting and joining some voice channel!")
		return
	}

	guild, err := s.Guild(gid)
	if err != nil {
		fmt.Println("Error while fetching guild: ", err)
		return
	}

	// Invoked from Server (Guild)

	if !strings.HasPrefix(m.Content, "!teams") {
		return
	}

	args := strings.Split(m.Content, " ")
	if len(args) <= 2 {
		sendReply(s, m, "Usage: `!teams <number of teams to create> -<game you want to create teams for> [skip username ...]`")
		return
	}


	_nTeams, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil {
		fmt.Println("Error while parsing user specified value: ", err)
		sendReply(s, m, "Please specify in number!!!")
		return
	}

	nTeams := int(_nTeams)

	if nTeams > len(teamsNames) {
		sendReply(s, m, "Actually there are only " + strconv.Itoa(len(teamsNames)) + " registered : " + strings.Join(teamsNames, ", "))
		return
	}

	if nTeams <= 0 || nTeams >= 100 {
		if gid == "223518751650217994" {
			// for internal uses
			sendReply(s, m, "<:kakattekoi:461046115257679872>")
		} else {
			sendReply(s, m, "Please specify in *realistic* number!!!!!")
		}
		return
	}


	var skipUsernames []string
	if len(args) > 3 {
		skipUsernames = args[3:len(args)]
	}

	if args[2][0] != '-' {
		sendReply(s, m, "You need to specify the game you want to build teams to")
		return
	}
	if args[2][1:] == "v" {
		createTeamValorant(s, m, guild, nTeams, skipUsernames)
	} else {
		sendReply(s, m, "Games actually supported : " + strings.Join(supportedGames, ", "))
		return
	}
}

func shuffleList(baseList []string) []string {
	var shuffledList []string

	// shuffle by connected users
	idx := rand.Perm(len(baseList))

	for _, newIdx := range idx {
		shuffledList = append(shuffledList, baseList[newIdx])
	}
	return shuffledList
}


// Different games

func createTeamValorant(s *discordgo.Session, m *discordgo.MessageCreate, guild *discordgo.Guild, nTeams int, skipUsernames []string) {
	// find users voice channel & fetch connected users
	voiceChannelUsers := map[string][]string{}
	var sourceVoiceChannel string
	for _, vs := range guild.VoiceStates {
		if vs.UserID == m.Author.ID {
			sourceVoiceChannel = vs.ChannelID
		}

		// check cache
		user, ok := usernames[vs.UserID]
		if !ok {
			// cache MISS
			u, err := s.User(vs.UserID)
			if err != nil {
				fmt.Println("Error while fetching username")
				sendReply(s, m, "Error: unknown error.")
				return
			}
			user = *u
			usernames[vs.UserID] = user
		}

		if !isContain(user.Username, skipUsernames) {
			voiceChannelUsers[vs.ChannelID] =
				append(voiceChannelUsers[vs.ChannelID], user.Username)
		}
	}

	// not found in any voice channel
	if sourceVoiceChannel == "" {
		sendReply(s, m, "Please connect some voice channel!")
		return
	}

	// check nTeams
	totalUserCount := len(voiceChannelUsers[sourceVoiceChannel])

	nMembers := int(math.Round(float64(totalUserCount) / float64(nTeams)))
	if totalUserCount < nTeams {
		sendReply(s, m, fmt.Sprintf("More member required to make %d team(s) by %d member(s)!", nTeams, nMembers))
		return
	}



	var shuffledUsers []string
	shuffledUsers = shuffleList(voiceChannelUsers[sourceVoiceChannel])

	// devide into {nTeams} teams
	result := make([][]string, nTeams)
	for i := 0; i < nTeams-1; i++ {
		result[i] = shuffledUsers[i*nMembers : (i+1)*nMembers]
	}
	result[nTeams-1] = shuffledUsers[(nTeams-1)*nMembers : len(shuffledUsers)]
	fmt.Println(result)

	// send message
	outputString := fmt.Sprintf("created %d team(s) for a Valorant game!\n", nTeams)
	for i := 0; i < nTeams; i++ {
		outputString += fmt.Sprintf(sides[i % 2] + " :: Team %s: %s\n", teamsNames[i], strings.Join(result[i], ", "))
	}
	outputString += fmt.Sprintf("Map order : %s \n", strings.Join(shuffleList(valorantMaps), ", "))

	sendReply(s, m, outputString)
}