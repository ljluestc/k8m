package pod

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weibaohui/k8m/pkg/comm/utils/amis"
)

// MockCluster is a mock implementation for testing
type MockCluster struct {
	selectedCluster string
}

func (m *MockCluster) GetSelectedCluster(c *gin.Context) (string, error) {
	return m.selectedCluster, nil
}

// TestBatchUpload tests the batch upload functionality
func TestBatchUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name           string
		files          []TestFile
		expectedStatus int
		expectError    bool
	}{
		{
			name: "Single file upload",
			files: []TestFile{
				{Name: "test1.txt", Content: "Hello World"},
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name: "Multiple files upload",
			files: []TestFile{
				{Name: "test1.txt", Content: "Hello World 1"},
				{Name: "test2.txt", Content: "Hello World 2"},
				{Name: "test3.json", Content: `{"key": "value"}`},
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name: "Empty files list",
			files: []TestFile{},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name: "Too many files",
			files: generateTestFiles(51), // Exceeds limit of 50
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name: "Large files",
			files: []TestFile{
				{Name: "large.txt", Content: strings.Repeat("A", 1024*1024)}, // 1MB file
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			router := gin.New()
			api := router.Group("/k8s")
			
			// Mock the amis.GetSelectedCluster function
			originalGetSelectedCluster := amis.GetSelectedCluster
			amis.GetSelectedCluster = func(c *gin.Context) (string, error) {
				return "test-cluster", nil
			}
			defer func() {
				amis.GetSelectedCluster = originalGetSelectedCluster
			}()

			RegisterPodFileRoutes(api)

			// Create multipart form data
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			// Add form fields
			writer.WriteField("containerName", "test-container")
			writer.WriteField("namespace", "test-namespace")
			writer.WriteField("podName", "test-pod")
			writer.WriteField("path", "/tmp")

			// Add files
			for _, file := range tt.files {
				part, err := writer.CreateFormFile("files", file.Name)
				require.NoError(t, err)
				_, err = part.Write([]byte(file.Content))
				require.NoError(t, err)
			}

			err := writer.Close()
			require.NoError(t, err)

			// Create request
			req := httptest.NewRequest("POST", "/k8s/file/batch-upload", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set("Authorization", "Bearer test-token")

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			router.ServeHTTP(w, req)

			// Check response
			assert.Equal(t, tt.expectedStatus, w.Code)

			if !tt.expectError {
				// Parse response
				var result map[string]interface{}
				err = json.Unmarshal(w.Body.Bytes(), &result)
				require.NoError(t, err)

				// Check response structure
				data, ok := result["data"].(map[string]interface{})
				require.True(t, ok)

				totalFiles, ok := data["total_files"].(float64)
				require.True(t, ok)
				assert.Equal(t, float64(len(tt.files)), totalFiles)

				successCount, ok := data["success_count"].(float64)
				require.True(t, ok)
				assert.Equal(t, float64(len(tt.files)), successCount)

				failureCount, ok := data["failure_count"].(float64)
				require.True(t, ok)
				assert.Equal(t, float64(0), failureCount)

				// Check individual file results
				files, ok := data["files"].([]interface{})
				require.True(t, ok)
				assert.Equal(t, len(tt.files), len(files))
			}
		})
	}
}

// TestBatchUploadConcurrency tests concurrent batch uploads
func TestBatchUploadConcurrency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	api := router.Group("/k8s")
	
	// Mock the amis.GetSelectedCluster function
	originalGetSelectedCluster := amis.GetSelectedCluster
	amis.GetSelectedCluster = func(c *gin.Context) (string, error) {
		return "test-cluster", nil
	}
	defer func() {
		amis.GetSelectedCluster = originalGetSelectedCluster
	}()

	RegisterPodFileRoutes(api)

	// Test concurrent uploads
	numConcurrent := 5
	done := make(chan bool, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func(index int) {
			defer func() { done <- true }()

			// Create test files
			files := []TestFile{
				{Name: fmt.Sprintf("concurrent_test_%d.txt", index), Content: fmt.Sprintf("Content %d", index)},
			}

			// Create multipart form data
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			// Add form fields
			writer.WriteField("containerName", "test-container")
			writer.WriteField("namespace", "test-namespace")
			writer.WriteField("podName", "test-pod")
			writer.WriteField("path", "/tmp")

			// Add files
			for _, file := range files {
				part, err := writer.CreateFormFile("files", file.Name)
				require.NoError(t, err)
				_, err = part.Write([]byte(file.Content))
				require.NoError(t, err)
			}

			err := writer.Close()
			require.NoError(t, err)

			// Create request
			req := httptest.NewRequest("POST", "/k8s/file/batch-upload", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set("Authorization", "Bearer test-token")

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			router.ServeHTTP(w, req)

			// Check response
			assert.Equal(t, http.StatusOK, w.Code)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numConcurrent; i++ {
		<-done
	}
}

// TestBatchUploadErrorHandling tests error handling scenarios
func TestBatchUploadErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name           string
		formData       map[string]string
		expectedStatus int
		expectedError  string
	}{
		{
			name: "Missing containerName",
			formData: map[string]string{
				"namespace": "test-namespace",
				"podName":   "test-pod",
				"path":      "/tmp",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "缺少必要参数",
		},
		{
			name: "Missing namespace",
			formData: map[string]string{
				"containerName": "test-container",
				"podName":       "test-pod",
				"path":          "/tmp",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "缺少必要参数",
		},
		{
			name: "Missing podName",
			formData: map[string]string{
				"containerName": "test-container",
				"namespace":     "test-namespace",
				"path":          "/tmp",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "缺少必要参数",
		},
		{
			name: "Missing path",
			formData: map[string]string{
				"containerName": "test-container",
				"namespace":     "test-namespace",
				"podName":       "test-pod",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "缺少必要参数",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			api := router.Group("/k8s")
			
			// Mock the amis.GetSelectedCluster function
			originalGetSelectedCluster := amis.GetSelectedCluster
			amis.GetSelectedCluster = func(c *gin.Context) (string, error) {
				return "test-cluster", nil
			}
			defer func() {
				amis.GetSelectedCluster = originalGetSelectedCluster
			}()

			RegisterPodFileRoutes(api)

			// Create multipart form data
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			// Add form fields
			for key, value := range tt.formData {
				writer.WriteField(key, value)
			}

			err := writer.Close()
			require.NoError(t, err)

			// Create request
			req := httptest.NewRequest("POST", "/k8s/file/batch-upload", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set("Authorization", "Bearer test-token")

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			router.ServeHTTP(w, req)

			// Check response
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedError)
		})
	}
}

// TestSaveUploadedFile tests the saveUploadedFile function
func TestSaveUploadedFile(t *testing.T) {
	// Create a test file
	testContent := "Hello, World!"
	testFileName := "test.txt"

	// Create multipart file header
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", testFileName)
	require.NoError(t, err)
	_, err = part.Write([]byte(testContent))
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	// Parse multipart form
	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	err = req.ParseMultipartForm(32 << 20)
	require.NoError(t, err)

	fileHeader := req.MultipartForm.File["file"][0]

	// Test saveUploadedFile
	tempFilePath, err := saveUploadedFile(fileHeader)
	require.NoError(t, err)
	defer os.RemoveAll(filepath.Dir(tempFilePath)) // Clean up

	// Verify file was saved correctly
	content, err := os.ReadFile(tempFilePath)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
	assert.Equal(t, testFileName, filepath.Base(tempFilePath))
}

// TestBatchUploadPerformance tests performance with large number of files
func TestBatchUploadPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	api := router.Group("/k8s")
	
	// Mock the amis.GetSelectedCluster function
	originalGetSelectedCluster := amis.GetSelectedCluster
	amis.GetSelectedCluster = func(c *gin.Context) (string, error) {
		return "test-cluster", nil
	}
	defer func() {
		amis.GetSelectedCluster = originalGetSelectedCluster
	}()

	RegisterPodFileRoutes(api)

	// Test with maximum allowed files (50)
	files := generateTestFiles(50)

	// Create multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add form fields
	writer.WriteField("containerName", "test-container")
	writer.WriteField("namespace", "test-namespace")
	writer.WriteField("podName", "test-pod")
	writer.WriteField("path", "/tmp")

	// Add files
	for _, file := range files {
		part, err := writer.CreateFormFile("files", file.Name)
		require.NoError(t, err)
		_, err = part.Write([]byte(file.Content))
		require.NoError(t, err)
	}

	err := writer.Close()
	require.NoError(t, err)

	// Measure performance
	start := time.Now()

	// Create request
	req := httptest.NewRequest("POST", "/k8s/file/batch-upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer test-token")

	// Create response recorder
	w := httptest.NewRecorder()

	// Perform request
	router.ServeHTTP(w, req)

	duration := time.Since(start)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Performance assertion - should complete within reasonable time
	assert.Less(t, duration, 10*time.Second, "Batch upload should complete within 10 seconds")

	t.Logf("Batch upload of %d files took %v", len(files), duration)
}

// Helper types and functions

type TestFile struct {
	Name    string
	Content string
}

func generateTestFiles(count int) []TestFile {
	files := make([]TestFile, count)
	for i := 0; i < count; i++ {
		files[i] = TestFile{
			Name:    fmt.Sprintf("test_file_%d.txt", i),
			Content: fmt.Sprintf("Content for file %d", i),
		}
	}
	return files
}

// BenchmarkBatchUpload benchmarks the batch upload performance
func BenchmarkBatchUpload(b *testing.B) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	api := router.Group("/k8s")
	
	// Mock the amis.GetSelectedCluster function
	originalGetSelectedCluster := amis.GetSelectedCluster
	amis.GetSelectedCluster = func(c *gin.Context) (string, error) {
		return "test-cluster", nil
	}
	defer func() {
		amis.GetSelectedCluster = originalGetSelectedCluster
	}()

	RegisterPodFileRoutes(api)

	// Generate test files
	files := generateTestFiles(10) // Use 10 files for benchmark

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Create multipart form data
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add form fields
		writer.WriteField("containerName", "test-container")
		writer.WriteField("namespace", "test-namespace")
		writer.WriteField("podName", "test-pod")
		writer.WriteField("path", "/tmp")

		// Add files
		for _, file := range files {
			part, err := writer.CreateFormFile("files", file.Name)
			if err != nil {
				b.Fatal(err)
			}
			_, err = part.Write([]byte(file.Content))
			if err != nil {
				b.Fatal(err)
			}
		}

		err := writer.Close()
		if err != nil {
			b.Fatal(err)
		}

		// Create request
		req := httptest.NewRequest("POST", "/k8s/file/batch-upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer test-token")

		// Create response recorder
		w := httptest.NewRecorder()

		// Perform request
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

