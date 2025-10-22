package pod

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/duke-git/lancet/v2/slice"
	"github.com/gin-gonic/gin"
	"github.com/weibaohui/k8m/pkg/comm/utils"
	"github.com/weibaohui/k8m/pkg/comm/utils/amis"
	"github.com/weibaohui/kom/kom"
	"k8s.io/klog/v2"
)

type FileController struct{}

// BatchUploadResult represents the result of a batch upload operation
type BatchUploadResult struct {
	TotalFiles   int                    `json:"total_files"`
	SuccessCount int                    `json:"success_count"`
	FailureCount int                    `json:"failure_count"`
	Files        []FileUploadResult     `json:"files"`
	Duration     time.Duration          `json:"duration"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      time.Time              `json:"end_time"`
}

// FileUploadResult represents the result of a single file upload
type FileUploadResult struct {
	FileName string `json:"file_name"`
	Status   string `json:"status"` // "done", "error"
	Error    string `json:"error,omitempty"`
	Size     int64  `json:"size"`
}

func RegisterPodFileRoutes(api *gin.RouterGroup) {
	ctrl := &FileController{}
	api.POST("/file/list", ctrl.List)
	api.POST("/file/show", ctrl.Show)
	api.POST("/file/save", ctrl.Save)
	api.GET("/file/download", ctrl.Download)
	api.POST("/file/upload", ctrl.Upload)
	api.POST("/file/batch-upload", ctrl.BatchUpload) // New batch upload endpoint
	api.POST("/file/delete", ctrl.Delete)
}

type info struct {
	ContainerName string `json:"containerName,omitempty"`
	PodName       string `json:"podName,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	IsDir         bool   `json:"isDir,omitempty"`
	Path          string `json:"path,omitempty"`
	FileContext   string `json:"fileContext,omitempty"`
	FileName      string `json:"fileName,omitempty"`
	Size          int64  `json:"size,omitempty"`
	FileType      string `json:"type,omitempty"` // 只有file类型可以查、下载
}

// BatchUpload 处理批量上传文件的 HTTP 请求
// @Summary 批量上传文件
// @Security BearerAuth
// @Param cluster query string true "集群名称"
// @Param containerName formData string true "容器名称"
// @Param namespace formData string true "命名空间"
// @Param podName formData string true "Pod名称"
// @Param path formData string true "文件路径"
// @Param files formData file true "上传文件列表"
// @Success 200 {object} BatchUploadResult
// @Router /k8s/cluster/{cluster}/file/batch-upload [post]
func (fc *FileController) BatchUpload(c *gin.Context) {
	selectedCluster, err := amis.GetSelectedCluster(c)
	if err != nil {
		amis.WriteJsonError(c, err)
		return
	}

	info := &info{}
	info.ContainerName = c.PostForm("containerName")
	info.Namespace = c.PostForm("namespace")
	info.PodName = c.PostForm("podName")
	info.Path = c.PostForm("path")

	if info.ContainerName == "" || info.Namespace == "" || info.PodName == "" || info.Path == "" {
		amis.WriteJsonError(c, fmt.Errorf("缺少必要参数: containerName, namespace, podName, path"))
		return
	}

	// 获取上传的文件列表
	form, err := c.MultipartForm()
	if err != nil {
		amis.WriteJsonError(c, fmt.Errorf("获取上传文件错误: %v", err))
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		amis.WriteJsonError(c, fmt.Errorf("没有找到上传的文件"))
		return
	}

	// 限制批量上传文件数量
	maxFiles := 50
	if len(files) > maxFiles {
		amis.WriteJsonError(c, fmt.Errorf("批量上传文件数量不能超过 %d 个", maxFiles))
		return
	}

	ctx := amis.GetContextWithUser(c)
	result := fc.processBatchUpload(ctx, selectedCluster, info, files)

	amis.WriteJsonData(c, result)
}

// processBatchUpload 处理批量上传逻辑
func (fc *FileController) processBatchUpload(ctx context.Context, selectedCluster string, info *info, files []*multipart.FileHeader) BatchUploadResult {
	startTime := time.Now()
	result := BatchUploadResult{
		TotalFiles: len(files),
		Files:      make([]FileUploadResult, len(files)),
		StartTime:  startTime,
	}

	// 并发控制 - 最多同时处理5个文件
	semaphore := make(chan struct{}, 5)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, file := range files {
		wg.Add(1)
		go func(index int, f *multipart.FileHeader) {
			defer wg.Done()
			
			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			fileResult := fc.uploadSingleFile(ctx, selectedCluster, info, f)
			
			// 线程安全地更新结果
			mu.Lock()
			result.Files[index] = fileResult
			if fileResult.Status == "done" {
				result.SuccessCount++
			} else {
				result.FailureCount++
			}
			mu.Unlock()
		}(i, file)
	}

	wg.Wait()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	klog.V(4).Infof("Batch upload completed: %d total, %d success, %d failed, duration: %v", 
		result.TotalFiles, result.SuccessCount, result.FailureCount, result.Duration)

	return result
}

// uploadSingleFile 上传单个文件
func (fc *FileController) uploadSingleFile(ctx context.Context, selectedCluster string, info *info, file *multipart.FileHeader) FileUploadResult {
	fileResult := FileUploadResult{
		FileName: file.Filename,
		Size:     file.Size,
	}

	// 验证文件名
	if file.Filename == "" {
		fileResult.Status = "error"
		fileResult.Error = "文件名不能为空"
		return fileResult
	}

	// 替换文件名中非法字符
	sanitizedFileName := utils.SanitizeFileName(file.Filename)
	if sanitizedFileName != file.Filename {
		klog.V(4).Infof("Sanitized filename: %s -> %s", file.Filename, sanitizedFileName)
	}

	// 保存上传文件到临时位置
	tempFilePath, err := saveUploadedFile(file)
	if err != nil {
		fileResult.Status = "error"
		fileResult.Error = fmt.Sprintf("保存临时文件失败: %v", err)
		return fileResult
	}
	defer os.Remove(tempFilePath) // 确保清理临时文件

	// 创建新的info结构用于单个文件上传
	singleFileInfo := &info{
		ContainerName: info.ContainerName,
		Namespace:     info.Namespace,
		PodName:       info.PodName,
		Path:          filepath.Join(info.Path, sanitizedFileName),
		FileName:      sanitizedFileName,
	}

	// 上传文件到 Pod
	if err := uploadToPod(ctx, selectedCluster, singleFileInfo, tempFilePath); err != nil {
		fileResult.Status = "error"
		fileResult.Error = fmt.Sprintf("上传到Pod失败: %v", err)
		return fileResult
	}

	fileResult.Status = "done"
	return fileResult
}

// List  处理获取文件列表的 HTTP 请求
// @Summary 获取文件列表
// @Security BearerAuth
// @Param cluster query string true "集群名称"
// @Param body body info true "文件信息"
// @Success 200 {object} string
// @Router /k8s/cluster/{cluster}/file/list [post]
func (fc *FileController) List(c *gin.Context) {
	selectedCluster, err := amis.GetSelectedCluster(c)
	if err != nil {
		amis.WriteJsonError(c, err)
		return
	}

	info := &info{}
	err = c.ShouldBindBodyWithJSON(info)
	if err != nil {
		amis.WriteJsonError(c, err)
		return
	}
	ctx := amis.GetContextWithUser(c)
	poder := kom.Cluster(selectedCluster).WithContext(ctx).
		Namespace(info.Namespace).
		Name(info.PodName).Ctl().Pod().
		ContainerName(info.ContainerName)

	if info.Path == "" {
		info.Path = "/"
	}
	// 获取文件列表
	nodes, err := poder.ListAllFiles(info.Path)
	if err != nil {
		amis.WriteJsonError(c, fmt.Errorf("获取文件列表失败,容器内没有shell或者没有ls命令"))
		return
	}
	// 作为文件树，应该去掉. .. 两个条目
	nodes = slice.Filter(nodes, func(index int, item *kom.FileInfo) bool {
		return item.Name != "." && item.Name != ".."
	})
	amis.WriteJsonList(c, nodes)
}

// Show 处理下载文件的 HTTP 请求
// @Summary 查看文件内容
// @Security BearerAuth
// @Param cluster query string true "集群名称"
// @Param body body info true "文件信息"
// @Success 200 {object} string
// @Router /k8s/cluster/{cluster}/file/show [post]
func (fc *FileController) Show(c *gin.Context) {
	selectedCluster, err := amis.GetSelectedCluster(c)
	if err != nil {
		amis.WriteJsonError(c, err)
		return
	}

	info := &info{}
	err = c.ShouldBindBodyWithJSON(info)
	if err != nil {
		amis.WriteJsonError(c, err)
		return
	}

	ctx := amis.GetContextWithUser(c)
	poder := kom.Cluster(selectedCluster).WithContext(ctx).
		Namespace(info.Namespace).
		Name(info.PodName).Ctl().Pod().
		ContainerName(info.ContainerName)
	if info.FileType != "" && info.FileType != "file" && info.FileType != "directory" {
		amis.WriteJsonError(c, fmt.Errorf("无法查看%s类型文件", info.FileType))
		return
	}
	if info.Path == "" {
		amis.WriteJsonError(c, fmt.Errorf("路径不能为空"))
		return
	}
	if info.IsDir {
		amis.WriteJsonError(c, fmt.Errorf("无法保存目录"))
		return
	}

	// 从容器中下载文件
	fileContent, err := poder.DownloadFile(info.Path)
	if err != nil {
		amis.WriteJsonError(c, err)
		return
	}
	isText, err := utils.IsTextFile(fileContent)
	if err != nil {
		amis.WriteJsonError(c, err)
		return
	}
	if !isText {
		amis.WriteJsonError(c, fmt.Errorf("%s包含非文本内容，请下载后查看", info.Path))
		return
	}

	amis.WriteJsonData(c, gin.H{
		"content": string(fileContent),
	})
}

// @Summary 保存文件
// @Security BearerAuth
// @Param cluster query string true "集群名称"
// @Param body body info true "文件信息"
// @Success 200 {object} string
// @Router /k8s/cluster/{cluster}/file/save [post]
func (fc *FileController) Save(c *gin.Context) {
	selectedCluster, err := amis.GetSelectedCluster(c)
	if err != nil {
		amis.WriteJsonError(c, err)
		return
	}

	info := &info{}
	err = c.ShouldBindBodyWithJSON(info)
	if err != nil {
		amis.WriteJsonError(c, err)
		return
	}
	klog.V(6).Infof("info \n%v\n", utils.ToJSON(info))

	ctx := amis.GetContextWithUser(c)
	poder := kom.Cluster(selectedCluster).WithContext(ctx).
		Namespace(info.Namespace).
		Name(info.PodName).Ctl().Pod().
		ContainerName(info.ContainerName)

	if info.Path == "" {
		amis.WriteJsonError(c, fmt.Errorf("路径不能为空"))
		return
	}
	if info.IsDir {
		amis.WriteJsonError(c, fmt.Errorf("无法保存目录"))
		return
	}

	// 上传文件
	if err := poder.SaveFile(info.Path, info.FileContext); err != nil {
		klog.V(6).Infof("Error uploading file: %v", err)
		amis.WriteJsonError(c, err)
		return
	}

	amis.WriteJsonOK(c)
}

// @Summary 下载文件
// @Security BearerAuth
// @Param cluster query string true "集群名称"
// @Param podName query string true "Pod名称"
// @Param path query string true "文件路径"
// @Param containerName query string true "容器名称"
// @Param namespace query string true "命名空间"
// @Success 200 {object} string
// @Router /k8s/cluster/{cluster}/file/download [get]
func (fc *FileController) Download(c *gin.Context) {
	selectedCluster, err := amis.GetSelectedCluster(c)
	if err != nil {
		amis.WriteJsonError(c, err)
		return
	}

	info := &info{}
	info.PodName = c.Query("podName")
	info.Path = c.Query("path")
	info.ContainerName = c.Query("containerName")
	info.Namespace = c.Query("namespace")

	ctx := amis.GetContextWithUser(c)
	poder := kom.Cluster(selectedCluster).WithContext(ctx).
		Namespace(info.Namespace).
		Name(info.PodName).Ctl().Pod().
		ContainerName(info.ContainerName)

	// 从容器中下载文件
	var fileContent []byte
	var finalFileName string
	if c.Query("type") == "tar" {
		fileContent, err = poder.DownloadTarFile(info.Path)
		// 从路径中提取文件名作为下载时的文件名，并添加.tar后缀
		fileName := filepath.Base(info.Path)
		fileNameWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		finalFileName = fileNameWithoutExt + ".tar"
	} else {
		fileContent, err = poder.DownloadFile(info.Path)
		finalFileName = filepath.Base(info.Path)
	}
	if err != nil {
		klog.V(6).Infof("下载文件错误: %v", err)
		amis.WriteJsonError(c, err)
		return
	}
	// 设置响应头，指定文件名和类型
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", finalFileName))
	c.Data(http.StatusOK, "application/octet-stream", fileContent)

}

// Upload 处理上传文件的 HTTP 请求
// @Summary 上传文件
// @Security BearerAuth
// @Param cluster query string true "集群名称"
// @Param containerName formData string true "容器名称"
// @Param namespace formData string true "命名空间"
// @Param podName formData string true "Pod名称"
// @Param path formData string true "文件路径"
// @Param fileName formData string true "文件名"
// @Param file formData file true "上传文件"
// @Success 200 {object} string
// @Router /k8s/cluster/{cluster}/file/upload [post]
func (fc *FileController) Upload(c *gin.Context) {
	selectedCluster, err := amis.GetSelectedCluster(c)
	if err != nil {
		amis.WriteJsonError(c, err)
		return
	}

	info := &info{}

	info.ContainerName = c.PostForm("containerName")
	info.Namespace = c.PostForm("namespace")
	info.PodName = c.PostForm("podName")
	info.Path = c.PostForm("path")
	info.FileName = c.PostForm("fileName")

	if info.FileName == "" {
		amis.WriteJsonData(c, gin.H{
			"file": gin.H{
				"uid":    -1,
				"name":   info.FileName,
				"status": "error",
				"error":  "文件名不能为空",
			},
		})
		return
	}
	if info.Path == "" {
		amis.WriteJsonData(c, gin.H{
			"file": gin.H{
				"uid":    -1,
				"name":   info.FileName,
				"status": "error",
				"error":  "路径不能为空",
			},
		})
		return
	}
	// 替换FileName中非法字符
	info.FileName = utils.SanitizeFileName(info.FileName)

	ctx := amis.GetContextWithUser(c)
	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		amis.WriteJsonData(c, gin.H{
			"file": gin.H{
				"uid":    -1,
				"name":   info.FileName,
				"status": "error",
				"error":  "获取上传文件错误",
			},
		})
		return
	}

	// 保存上传文件
	tempFilePath, err := saveUploadedFile(file)
	if err != nil {
		amis.WriteJsonData(c, gin.H{
			"file": gin.H{
				"uid":    -1,
				"name":   info.FileName,
				"status": "error",
				"error":  err.Error(),
			},
		})
		return
	}
	defer os.Remove(tempFilePath) // 请求结束时删除临时文件

	// 上传文件到 Pod 中
	if err := uploadToPod(ctx, selectedCluster, info, tempFilePath); err != nil {
		amis.WriteJsonData(c, gin.H{
			"file": gin.H{
				"uid":    -1,
				"name":   info.FileName,
				"status": "error",
				"error":  err.Error(),
			},
		})
		return
	}

	// 	{
	//    uid: 'uid',      // 文件唯一标识，建议设置为负数，防止和内部产生的 id 冲突
	//    name: 'xx.png',   // 文件名
	//    status: 'done' | 'uploading' | 'error' | 'removed' , //  beforeUpload 拦截的文件没有 status 状态属性
	//    response: '{"status": "success"}', // 服务端响应内容
	//    linkProps: '{"download": "image"}', // 下载链接额外的 HTML 属性
	// }
	amis.WriteJsonData(c, gin.H{
		"file": gin.H{
			"uid":    -1,
			"name":   info.FileName,
			"status": "done",
		},
	})

}

// @Summary 删除文件
// @Security BearerAuth
// @Param cluster query string true "集群名称"
// @Param body body info true "文件信息"
// @Success 200 {object} string
// @Router /k8s/cluster/{cluster}/file/delete [post]
func (fc *FileController) Delete(c *gin.Context) {
	selectedCluster, err := amis.GetSelectedCluster(c)
	if err != nil {
		amis.WriteJsonError(c, err)
		return
	}

	info := &info{}
	err = c.ShouldBindBodyWithJSON(info)
	if err != nil {
		amis.WriteJsonError(c, err)
		return
	}

	ctx := amis.GetContextWithUser(c)
	poder := kom.Cluster(selectedCluster).WithContext(ctx).
		Namespace(info.Namespace).
		Name(info.PodName).Ctl().Pod().
		ContainerName(info.ContainerName)
	// 从容器中下载文件
	result, err := poder.DeleteFile(info.Path)
	if err != nil {
		klog.V(6).Infof("删除文件错误: %v", err)
		amis.WriteJsonError(c, err)
		return
	}

	amis.WriteJsonOKMsg(c, "删除成功"+string(result))
}

// saveUploadedFile 保存上传文件并返回临时文件路径
func saveUploadedFile(file *multipart.FileHeader) (string, error) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "upload-*")
	if err != nil {
		return "", fmt.Errorf("创建临时目录错误: %v", err)
	}

	// 使用原始文件名生成临时文件路径
	tempFilePath := filepath.Join(tempDir, file.Filename)

	// 创建并保存文件
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return "", fmt.Errorf("创建临时文件错误: %v", err)
	}
	defer tempFile.Close()

	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("打开上传文件错误: %v", err)
	}
	defer src.Close()

	if _, err := io.Copy(tempFile, src); err != nil {
		return "", fmt.Errorf("无法写入临时文件: %v", err)
	}

	return tempFilePath, nil
}

// uploadToPod 上传文件到 Pod
func uploadToPod(ctx context.Context, selectedCluster string, info *info, tempFilePath string) error {

	poder := kom.Cluster(selectedCluster).WithContext(ctx).
		Namespace(info.Namespace).
		Name(info.PodName).Ctl().Pod().
		ContainerName(info.ContainerName)

	openTmpFile, err := os.Open(tempFilePath)
	if err != nil {
		return fmt.Errorf("打开上传临时文件错误: %v", err)
	}
	defer openTmpFile.Close()

	// 上传文件到 Pod 中
	if err := poder.UploadFile(info.Path, openTmpFile); err != nil {
		return fmt.Errorf("上传文件到Pod中错误: %v", err)
	}

	return nil
}