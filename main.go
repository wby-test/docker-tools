package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

const (
	imagesSaveDirConst = "images-save"
	imagesLoadDirConst = imagesSaveDirConst
	oldRegistryConst   = "dockerhub.mlops.xx.com"
)

var (
	oldRegistry   string
	newRegistry   string
	imagesLoadDir string
	imageSaveDir  string
)

func main() {
	// 创建根命令
	rootCmd := &cobra.Command{
		Use:   "docker-tool",
		Short: "A tool to save, load and replace Docker images",
	}

	// 创建 save 子命令
	saveCmd := &cobra.Command{
		Use:   "save",
		Short: "Save All Docker images",
		Run: func(cmd *cobra.Command, args []string) {
			saveAllImages()
		},
	}

	//  加载 load 子命令
	loadCmd := &cobra.Command{
		Use:   "load",
		Short: "Load All Docker images",
		Run: func(cmd *cobra.Command, args []string) {
			loadAllImages()
		},
	}

	// 替换 replace 子命令
	replaceCmd := &cobra.Command{
		Use:   "replace",
		Short: "Replace All Docker images",
		Run: func(cmd *cobra.Command, args []string) {
			replaceAllImages()
		},
	}

	// 设置命令行参数
	replaceCmd.Flags().StringVar(&oldRegistry, "old", oldRegistryConst, "Old image registry")
	replaceCmd.Flags().StringVar(&newRegistry, "new", "", "New image registry")
	loadCmd.Flags().StringVar(&imagesLoadDir, "load", imagesLoadDirConst, "Images load Path")
	saveCmd.Flags().StringVar(&imageSaveDir, "save", imagesSaveDirConst, "Images save Path")
	rootCmd.AddCommand(saveCmd, loadCmd, replaceCmd)
	// 执行根命令
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getDockerClient() *client.Client {
	cliVersion, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.41"))
	if err != nil {
		log.Fatal(err)
	}

	version, err := cliVersion.ServerVersion(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion(version.APIVersion))
	if err != nil {
		log.Fatal(err)
	}

	return cli
}

func saveAllImages() {

	// 创建镜像存储的文件夹
	if err := os.MkdirAll(imageSaveDir, os.ModePerm); err != nil {
		log.Fatal(err)
	}

	cli := getDockerClient()
	images, err := cli.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		log.Fatal(err)
	}

	for _, image := range images {
		imageTags := image.RepoTags
		if len(imageTags) == 0 {
			continue
		}
		imageName := imageTags[0]
		imageNameParts := strings.Split(imageName, ":")

		if len(imageNameParts) != 2 {
			continue
		}

		imageName = strings.ReplaceAll(imageNameParts[0], "/", "-")
		imageTag := imageNameParts[1]

		filename := fmt.Sprintf("%s-%s.tar", imageName, imageTag)

		imageReader, err := cli.ImageSave(context.Background(), []string{image.ID})
		if err != nil {
			log.Fatal("Failed to save image %s: %s\n", imageName, err)
		}

		file, err := os.Create(imageSaveDir + "/" + filename)
		if err != nil {
			log.Fatal("Failed to create file %s: %s\n", filename, err)
		}
		defer file.Close()

		_, err = io.Copy(file, imageReader)
		if err != nil {
			log.Fatal("Failed to write data to file %s: %s\n", filename, err)
		}

		fmt.Printf("Image %s saved as %s\n", imageName, filename)
	}
}

func loadAllImages() {
	cli := getDockerClient()
	folderPath := imagesLoadDir

	folder, err := os.Open(folderPath)
	if err != nil {
		log.Fatal(err)
	}
	defer folder.Close()

	// 读取文件夹中的每个文件
	files, err := folder.Readdir(-1)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		// 拼接文件路径
		filePath := fmt.Sprintf("%s/%s", folderPath, file.Name())

		// 打开镜像文件
		imageFile, err := os.Open(filePath)
		if err != nil {
			log.Fatal(err)
		}
		defer imageFile.Close()

		// 调用Docker API加载镜像
		resp, err := cli.ImageLoad(context.Background(), imageFile, true)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		// 读取加载镜像的响应
		output := make([]byte, 4096)
		_, err = resp.Body.Read(output)
		if err != nil && err != io.EOF {
			log.Fatal(err)
		}
		fmt.Println(string(output))
	}
}

func replaceAllImages() {
	cli := getDockerClient()
	images, err := cli.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		log.Fatal(err)
	}

	for _, image := range images {
		for _, tag := range image.RepoTags {
			if !strings.Contains(tag, oldRegistry) {
				continue
			}
			newTag := strings.Replace(tag, oldRegistry, newRegistry, -1)
			err = cli.ImageTag(context.Background(), tag, newTag)
			if err != nil {
				panic(err)
			}
			fmt.Printf("镜像 [%s] 的域名已更新为 [%s]\n", tag, newTag)
		}
	}
}

func deleteAllImages() {
	cli := getDockerClient()
	images, err := cli.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		log.Fatal(err)
	}

	for _, image := range images {
		for _, tag := range image.RepoTags {
			if strings.Contains(tag, oldRegistry) {
				cli.ImageRemove(context.Background(), tag, types.ImageRemoveOptions{})
				fmt.Printf("镜像 [%s] 的域名已更新为 [%s]\n", tag, "")
			}
		}
	}
}
