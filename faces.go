// Package main implements faces app.
package main

import (
	"context"
	"embed"
	"flag"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/Kagami/go-face"
	"github.com/bool64/dev/version"
	"github.com/swaggest/jsonschema-go"
	"github.com/swaggest/openapi-go/openapi3"
	"github.com/swaggest/rest/web"
	swgui "github.com/swaggest/swgui/v5emb"
	"github.com/swaggest/usecase"
)

//go:embed models
var models embed.FS

func must[V any](v V, err error) V {
	if err != nil {
		panic(err)
	}

	return v
}

func main() {
	listen := flag.String("listen", "localhost:8011", "listen address")
	flag.Parse()

	start := time.Now()

	if _, err := os.Stat("./models/dlib_face_recognition_resnet_model_v1.dat"); err != nil {
		if os.IsNotExist(err) {
			if err := os.Mkdir("models", 0o700); err != nil && !os.IsExist(err) {
				log.Fatal(err)
			}

			for _, fn := range []string{
				"models/dlib_face_recognition_resnet_model_v1.dat",
				"models/mmod_human_face_detector.dat",
				"models/shape_predictor_5_face_landmarks.dat",
			} {
				f := must(models.Open(fn))
				d := must(os.Create(fn)) //nolint:gosec
				must(io.Copy(d, f))
				must(1, f.Close())
				must(1, d.Close())
			}
		} else {
			must(1, err)
		}
	}

	rec := must(face.NewRecognizer("./models"))
	defer rec.Close()

	log.Println("recognizer init", time.Since(start))

	r := openapi3.NewReflector()
	r.JSONSchemaReflector().DefaultOptions = append(r.JSONSchemaReflector().DefaultOptions, jsonschema.ProcessWithoutTags)

	s := web.NewService(r)

	// Init API documentation schema.
	s.OpenAPISchema().SetTitle("Faces Detector")
	s.OpenAPISchema().SetDescription("REST API to detect faces in images.")
	s.OpenAPISchema().SetVersion(version.Info().Version)

	s.Post("/image", uploadImage(rec))

	// Swagger UI endpoint at /docs.
	s.Docs("/docs", swgui.New)

	// Start server.
	log.Println("http://" + *listen + "/docs")
	server := &http.Server{
		Addr:              *listen,
		ReadHeaderTimeout: 3 * time.Second,
		Handler:           s,
	}

	if err := server.ListenAndServe(); err != nil {
		must(1, err)
	}
}

func uploadImage(rec *face.Recognizer) usecase.Interactor {
	type upload struct {
		Upload multipart.File `formData:"uploads" description:"JPG image."`
	}

	type output struct {
		ElapsedSec float64     `json:"elapsedSec"`
		Faces      []face.Face `json:"faces,omitempty"`
	}

	u := usecase.NewInteractor(func(ctx context.Context, in upload, out *output) (err error) {
		start := time.Now()
		imgData, err := io.ReadAll(in.Upload)
		if err != nil {
			return err
		}

		out.Faces, err = rec.Recognize(imgData)
		out.ElapsedSec = time.Since(start).Seconds()

		return err
	})

	u.SetTitle("Files Uploads With 'multipart/form-data'")

	return u
}
