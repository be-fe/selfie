package apps

import (
	"archive/zip"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/pressly/selfie/common"
	"github.com/pressly/selfie/data"
	internalErrors "github.com/pressly/selfie/errors"
	"github.com/pressly/selfie/logme"
	"github.com/pressly/selfie/web/util"

	"github.com/pressly/selfie/config"
	"github.com/pressly/selfie/lib/crypto"
	"github.com/pressly/selfie/lib/utils"

	"golang.org/x/net/context"
)

func copyDataToFile(in io.Reader, to string) error {
	out, err := os.Create(to)
	if err != nil {
		return err
	}

	defer out.Close()

	_, err = io.Copy(out, in)

	if err != nil {
		return err
	}

	return nil
}

func uuid() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println("Error: ", err)
		return ""
	}

	return fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func uploadBundles(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID, _ := util.GetUserIDFromContext(ctx)
	appID, _ := util.GetParamValueAsID(ctx, "appID")
	releaseID, _ := util.GetParamValueAsID(ctx, "releaseID")

	//check if user has permission admin or owner
	if !data.DB.App.HasPermission(appID, userID, data.ADMIN, data.OWNER) {
		utils.RespondEx(w, nil, 0, internalErrors.ErrorAuthorizeAccess)
		return
	}

	if err := r.ParseMultipartForm(config.Conf.FileUpload.MaxSize); err != nil {
		utils.RespondEx(w, nil, 0, internalErrors.ErrorSomethingWentWrong)
		logme.Warn(err.Error())
		return
	}

	var fileInfos []*common.FileInfo
	var path string

	fileInfos = make([]*common.FileInfo, 0)

	//saving all the files into temp folder
	for _, fileHeaders := range r.MultipartForm.File {
		for _, fileHeader := range fileHeaders {
			file, _ := fileHeader.Open()
			path = config.Conf.FileUpload.Temp + uuid()
			if err := copyDataToFile(file, path); err != nil {
				utils.RespondEx(w, nil, 0, internalErrors.ErrorSomethingWentWrong)
				logme.Warn(err.Error())
				return
			}

			file.Close()

			hash, err := crypto.HashFile(path)
			if err != nil {
				utils.RespondEx(w, nil, 0, internalErrors.ErrorSomethingWentWrong)
				logme.Warn(err.Error())
				return
			}

			fileInfos = append(fileInfos, &common.FileInfo{
				Filename:     fileHeader.Filename,
				Hash:         hash,
				TempLocation: path,
			})
		}
	}

	bundles, err := data.DB.Bundle.UploadBundles(releaseID, appID, userID, fileInfos)

	if err != nil {
		logme.Warn(err.Error())
		//remove all temp files
		for _, fileInfo := range fileInfos {
			os.Remove(fileInfo.TempLocation)
		}
	} else {
		for _, fileInfo := range fileInfos {
			err = os.Rename(fileInfo.TempLocation, config.Conf.FileUpload.Bundle+fileInfo.Hash)
			logme.Info(fileInfo.TempLocation)
			if err != nil {
				logme.Warn(err.Error())
			}
		}
	}

	utils.RespondEx(w, bundles, 0, err)
}

func getAllBundles(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID, _ := util.GetUserIDFromContext(ctx)
	appID, _ := util.GetParamValueAsID(ctx, "appID")
	releaseID, _ := util.GetParamValueAsID(ctx, "releaseID")

	bundles, err := data.DB.Bundle.FindAllBundles(releaseID, appID, userID)
	utils.RespondEx(w, bundles, 0, err)
}

func deleteBundle(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID, _ := util.GetUserIDFromContext(ctx)
	appID, _ := util.GetParamValueAsID(ctx, "appID")
	releaseID, _ := util.GetParamValueAsID(ctx, "releaseID")
	bundleID, _ := util.GetParamValueAsID(ctx, "bundleID")

	err := data.DB.Bundle.RemoveBundle(bundleID, releaseID, appID, userID)
	utils.RespondEx(w, nil, 0, err)
}

func downloadBundle(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID, _ := util.GetUserIDFromContext(ctx)
	appID, _ := util.GetParamValueAsID(ctx, "appID")
	releaseID, _ := util.GetParamValueAsID(ctx, "releaseID")

	bundles, err := data.DB.Bundle.FindAllBundles(releaseID, appID, userID)

	if err != nil {
		utils.RespondEx(w, nil, 0, err)
		return
	}

	//set the header to application/zip
	w.Header().Set("Content-Type", "application/zip")

	//zip the files
	compress := zip.NewWriter(w)
	defer compress.Close()

	type meta struct {
		Name string `json:"actual_file_name"`
		File string `json:"file_name"`
	}

	var bundleMeta []*meta
	bundleMeta = make([]*meta, 0)

	for _, bundle := range bundles {
		bundleMeta = append(bundleMeta, &meta{Name: bundle.Name, File: bundle.Hash})

		//create a file with hash name
		conatinFile, _ := compress.Create(bundle.Hash)
		targetFile, _ := os.Open(config.Conf.FileUpload.Bundle + bundle.Hash)

		io.Copy(conatinFile, targetFile)

		targetFile.Close()
	}

	//bundle meta information which maps every file inside the zip file
	bundle, _ := compress.Create("bundle.json")
	json.NewEncoder(bundle).Encode(bundleMeta)
}
