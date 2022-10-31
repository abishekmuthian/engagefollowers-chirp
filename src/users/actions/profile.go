package useractions

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/ioutil"
	"strconv"
	"strings"

	_ "embed"

	"github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	userModel "github.com/abishekmuthian/engagefollowers/src/users"
	"github.com/go-redis/redis/v8"
	"github.com/golang/freetype/truetype"
	"github.com/psykhi/wordclouds"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"gopkg.in/yaml.v2"
)

//go:embed config/config.yaml
var content []byte

//go:embed config/twitter_header_bg.png
var bgImage []byte

type MaskConf struct {
	File  string
	Color color.RGBA
}

type Conf struct {
	FontMaxSize     int    `yaml:"font_max_size"`
	FontMinSize     int    `yaml:"font_min_size"`
	NameFontSize    int    `yaml:"name_font_size"`
	RandomPlacement bool   `yaml:"random_placement"`
	NameFontFile    string `yaml:"name_font_file"`
	FontFile        string `yaml:"font_file"`
	Colors          []color.RGBA
	BackgroundColor color.Alpha16 `yaml:"background_color"`
	Width           int
	Height          int
	Mask            MaskConf
	SizeFunction    *string `yaml:"size_function"`
	Debug           bool
}

// GenerateProfileBanner generates profile banner for Twitter from classified hashtags
func GenerateProfileBanner() {

	var DefaultColors = []color.RGBA{
		{0x1b, 0x1b, 0x1b, 0xff},
		{0x48, 0x48, 0x4B, 0xff},
		{0x59, 0x3a, 0xee, 0xff},
		{0x65, 0xCD, 0xFA, 0xff},
		{0x70, 0xD6, 0xBF, 0xff},
	}

	var DefaultConf = Conf{
		FontMaxSize:     90,
		FontMinSize:     14,
		RandomPlacement: false,
		FontFile:        "./public/assets/fonts/SourceSans3-Regular.ttf",
		Colors:          DefaultColors,
		BackgroundColor: color.Transparent,
		Width:           1000,
		Height:          500,
		Mask: MaskConf{"", color.RGBA{
			R: 0,
			G: 0,
			B: 0,
			A: 0,
		}},
		Debug: false,
	}

	// Load config
	conf := DefaultConf

	err := yaml.Unmarshal(content, &conf)

	if err != nil {
		log.Error(log.V{"Profile, Failed to decode config, using defaults instead": err})
	}

	// Initialize redis
	var ctx = context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr:     config.Get("redis_server"),
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	q := userModel.Query()

	// User is not suspended
	q.Where("status=100")

	// User should not have trial ended or subscription should be true
	q.Where("trial_end IS NULL OR subscription is TRUE")

	// Fetch the userModel
	users, err := userModel.FindAll(q)
	if err != nil {
		log.Error(log.V{"message": "email: error getting users for checking tweets", "error": err})
		return
	}

	if len(users) > 0 {

		for _, user := range users {

			inputWords := make(map[string]int)

			if user.ProfileBanner && user.TwitterOauthConnected {
				for _, category := range user.Keywords {
					score, err := rdb.Get(ctx, config.Get("redis_key_prefix")+strconv.FormatInt(user.ID, 10)+":"+category+config.Get("redis_key_profile_banner_label_suffix")).Int()

					if err != nil {
						log.Error(log.V{"Profile, Error retrieving data from redis": err})

						if err.Error() == "redis: nil" {
							log.Info(log.V{"Profile": "No value found in redis"})
							inputWords[category] = 0
							continue
						}
					}

					log.Info(log.V{"Profile, Category": category, "Score": score})
					inputWords[category] = score
				}

				// Check if all the values are zero for input words
				var sum int = 0
				for _, val := range inputWords {
					sum += val
				}

				if sum == 0 {
					// All input words values are zero, Don't show this banner
					log.Info(log.V{"Profile": "All input words values are zero, Don't show this banner"})
					continue
				}

				// Generate word cloud image

				colors := make([]color.Color, 0)
				for _, c := range conf.Colors {
					colors = append(colors, c)
				}

				oarr := []wordclouds.Option{wordclouds.FontFile(conf.FontFile),
					wordclouds.FontMaxSize(conf.FontMaxSize),
					wordclouds.FontMinSize(conf.FontMinSize),
					wordclouds.Colors(colors),
					// wordclouds.MaskBoxes(boxes),
					wordclouds.Height(conf.Height),
					wordclouds.Width(conf.Width),
					wordclouds.RandomPlacement(conf.RandomPlacement),
					wordclouds.BackgroundColor(conf.BackgroundColor),
				}
				if conf.SizeFunction != nil {
					oarr = append(oarr, wordclouds.WordSizeFunction(*conf.SizeFunction))
				}
				if conf.Debug {
					oarr = append(oarr, wordclouds.Debug())
				}
				w := wordclouds.NewWordcloud(inputWords,
					oarr...,
				)

				img := w.Draw()

				/* bgImage, err := os.Open("public/assets/images/app/twitter_header_bg.png")
				if err != nil {
					log.Error(log.V{"Profile, Image generation, Failed to open": err})
				} */

				bg, err := png.Decode(bytes.NewBuffer(bgImage))
				if err != nil {
					log.Error(log.V{"Profile, Image generation, Failed to decode": err})
				}
				// defer bgImage.Close()

				offset := image.Pt(300, -10)
				b := bg.Bounds()
				headerImage := image.NewRGBA(b)
				draw.Draw(headerImage, b, bg, image.ZP, draw.Src)
				draw.Draw(headerImage, img.Bounds().Add(offset), img, image.ZP, draw.Over)

				/* 		header, err := os.Create("./public/assets/images/header.png")
				if err != nil {
					log.Error(log.V{"Profile, Image generation, Failed to create": err})
				}
				*/

				// Add name

				name := strings.Fields(user.TwitterName)

				addName(headerImage, 80, 200, name[0]+"!", conf)
				var header bytes.Buffer
				png.Encode(&header, headerImage)

				// 	        // Mime type results in media error on Twitter
				// 			// Prepend the appropriate URI scheme header depending
				// 			// on the MIME type
				// 			// Determine the content type of the image file
				// 			    mimeType := http.DetectContentType(bytes)
				// 			    switch mimeType {
				// 			   	case "image/jpeg":
				// 			   		base64Encoding += "data:image/jpeg;base64,"
				// 			   	case "image/png":
				// 			   		base64Encoding += "data:image/png;base64,"
				// 			   	} */

				// // Append the base64 encoded output
				base64Encoding := base64.StdEncoding.EncodeToString(header.Bytes())

				err = UpdateProfileBanner(user, base64Encoding)

				if err != nil {
					sendAdminEmail(user, config.Get("email_profile_banner_subject"), err.Error())
				} else {
					// Store the count for profile banner update
					// Increment the number of times this email has been sent
					rdb.IncrBy(ctx, config.Get("redis_key_prefix")+strconv.FormatInt(user.ID, 10)+config.Get("redis_key_profile_banner_email_suffix"), 1)

					// Inform the user first time
					emailCount, err := rdb.Get(ctx, config.Get("redis_key_prefix")+strconv.FormatInt(user.ID, 10)+config.Get("redis_key_profile_banner_email_suffix")).Int()

					if err != nil {
						log.Error(log.V{"Profile, Error retrieving profile banner count": err})
						continue
					} else {
						if emailCount == 1 {
							sendProfileBannerEmail(user, rdb, ctx)
						}
					}
				}
			} else {
				log.Info(log.V{"Profile, User hasn't enabled the profile banner (or) oauth1 token is not available": ""})
			}
		}

	}
}

// addName adds the name as a label to the header image
func addName(img *image.RGBA, x, y int, name string, conf Conf) {

	col := color.RGBA{255, 255, 255, 255}

	point := fixed.Point26_6{fixed.I(x), fixed.I(y)}

	// Read the font data.
	fontBytes, err := ioutil.ReadFile(conf.NameFontFile)
	if err != nil {
		log.Error(log.V{"Profile, Error in reading font file for setting name": err})
		return
	}
	f, err := truetype.Parse(fontBytes)
	if err != nil {
		log.Error(log.V{"Profile, Error in parsing font for setting name": err})
		return
	}

	// Draw the text.

	d := &font.Drawer{
		Dst: img,
		Src: image.NewUniform(col),
		Face: truetype.NewFace(f, &truetype.Options{
			Size:    float64(conf.NameFontSize),
			DPI:     72,
			Hinting: font.HintingNone,
		}),
		Dot: point,
	}
	d.DrawString(name)

}
