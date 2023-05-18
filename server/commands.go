package main

import (
	"bytes"
	"image/png"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

func (p *Plugin) registerCommands() {
	p.API.RegisterCommand(&model.Command{
		Trigger:          "summarize",
		DisplayName:      "Summarize",
		Description:      "Summarize current context",
		AutoComplete:     true,
		AutoCompleteDesc: "Summarize current context",
	})

	p.API.RegisterCommand(&model.Command{
		Trigger:          "imagine",
		DisplayName:      "Imagine",
		Description:      "Generate a new image based on the provided text",
		AutoComplete:     true,
		AutoCompleteDesc: "Generate a new image based on the provided text",
	})
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if args == nil {
		return nil, model.NewAppError("Summarize.ExecuteCommand", "app.command.execute.error", nil, "", http.StatusInternalServerError)
	}

	channel, err := p.pluginAPI.Channel.Get(args.ChannelId)
	if err != nil {
		return nil, model.NewAppError("Summarize.ExecuteCommand", "app.command.execute.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := p.checkUsageRestrictions(args.UserId, channel); err != nil {
		return nil, model.NewAppError("Summarize.ExecuteCommand", "Not authorized", nil, err.Error(), http.StatusUnauthorized)
	}

	split := strings.SplitN(strings.TrimSpace(args.Command), " ", 2)
	command := split[0]
	/*parameters := []string{}
	cmd := ""
	if len(split) > 1 {
		cmd = split[1]
	}
	if len(split) > 2 {
		parameters = split[2:]
	}*/

	if command != "/summarize" && command != "/imagine" {
		return &model.CommandResponse{}, nil
	}

	if command == "/summarize" {
		var response *model.CommandResponse
		var err error
		response, err = p.summarizeCurrentContext(c, args)

		if err != nil {
			return nil, model.NewAppError("Summarize.ExecuteCommand", "app.command.execute.error", nil, err.Error(), http.StatusInternalServerError)
		}
		return response, nil
	}

	if command == "/imagine" {
		prompt := strings.Join(split[1:], " ")
		if err := p.imagine(c, args, prompt); err != nil {
			return nil, model.NewAppError("Imagine.ExecuteCommand", "app.imagine.command.execute.error", nil, err.Error(), http.StatusInternalServerError)
		}
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "Generating image, please wait.",
			ChannelId:    args.ChannelId,
		}, nil
	}

	return &model.CommandResponse{}, nil
}

func (p *Plugin) summarizeCurrentContext(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, error) {
	if args.RootId != "" {
		postid, err := p.startNewSummaryThread(args.RootId, args.UserId)
		if err != nil {
			return nil, err
		}
		return &model.CommandResponse{
			GotoLocation: "/_redirect/pl/" + postid,
		}, nil
	}

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         "Channel summarization not implmented",
		ChannelId:    args.ChannelId,
	}, nil
}

func (p *Plugin) imagine(c *plugin.Context, args *model.CommandArgs, prompt string) error {
	go func() {
		imgBytes, err := p.imageGenerator.GenerateImage(prompt)
		if err != nil {
			p.API.LogError("Unable to generate the new image", "error", err)
			return
		}

		buf := new(bytes.Buffer)
		if err := png.Encode(buf, imgBytes); err != nil {
			p.API.LogError("Unable to parse image", "error", err)
			return
		}

		fileInfo, appErr := p.API.UploadFile(buf.Bytes(), args.ChannelId, "generated-image.png")
		if appErr != nil {
			p.API.LogError("Unable to upload the attachment", "error", appErr)
			return
		}

		_, appErr = p.API.CreatePost(&model.Post{
			Message:   "Image generated by the AI from the text: " + prompt,
			ChannelId: args.ChannelId,
			UserId:    args.UserId,
			FileIds:   []string{fileInfo.Id},
		})
		if appErr != nil {
			p.API.LogError("Unable to post the new message", "error", appErr)
			return
		}
	}()

	return nil
}
