import { message, Modal, Progress } from 'antd';
import { fetcher } from '@/components/Amis/fetcher.ts';
import { FileNode } from './FileTree';
import { ProcessK8sUrlWithCluster } from '@/utils/utils';

interface FileOperationsProps {
    selectedContainer: string;
    podName: string;
    namespace: string;
}

interface BatchUploadProgress {
    total: number;
    completed: number;
    failed: number;
    current: string;
    isVisible: boolean;
}

export class FileOperations {
    private props: FileOperationsProps;
    private batchUploadProgress: BatchUploadProgress = {
        total: 0,
        completed: 0,
        failed: 0,
        current: '',
        isVisible: false
    };

    constructor(props: FileOperationsProps) {
        this.props = props;
    }

    async handleCopy(node: FileNode) {
        await navigator.clipboard.writeText(node.path);
        message.success('路径已复制到剪贴板');
    }

    async handleRefresh(node: FileNode, callback: (children: FileNode[]) => void) {
        const children = await this.fetchData(node.path, node.isDir);
        if (node.isDir) {
            callback(children);
        }
        message.success('刷新成功');
    }

    async handleDelete(node: FileNode, onSuccess: () => void) {
        Modal.confirm({
            title: '请确认',
            content: `是否确认删除文件：${node.path} ？`,
            okText: '删除',
            cancelText: '取消',
            onOk: async () => {
                const response = await fetcher({
                    url: '/k8s/file/delete',
                    method: 'post',
                    data: {
                        containerName: this.props.selectedContainer,
                        podName: this.props.podName,
                        namespace: this.props.namespace,
                        path: node.path
                    }
                });
                message.success(response.data?.msg);
                if (response.data?.status === 0) {
                    onSuccess();
                }
            }
        });
    }

    async handleEditFile(node: FileNode, onEditorLoad: (content: string, language: string) => void) {
        if (node.type !== 'file') {
            message.error('只能编辑文件类型');
            return;
        }

        try {
            const response = await fetcher({
                url: '/k8s/file/show',
                method: 'post',
                data: {
                    containerName: this.props.selectedContainer,
                    podName: this.props.podName,
                    namespace: this.props.namespace,
                    path: node.path
                }
            });

            if (response.data?.status !== 0) {
                message.error(response.data?.msg || '非文本文件，不可在线编辑。请下载编辑后上传。');
                return;
            }

            //@ts-ignore
            const fileContent = response.data?.data?.content || '';
            let language = node.path?.split('.').pop() || 'plaintext';

            switch (language) {
                case 'yaml':
                case 'yml':
                    language = 'yaml';
                    break;
                case 'json':
                    language = 'json';
                    break;
                case 'py':
                    language = 'python';
                    break;
                default:
                    language = 'shell';
                    break;
            }

            onEditorLoad(fileContent, language);
        } catch (error) {
            message.error('获取文件内容失败');
        }
    }

    async downloadFile(node: FileNode, type?: string) {
        if (!node || !this.props.selectedContainer || !this.props.podName || !this.props.namespace) {
            message.error('缺少必要的参数，请检查输入');
            return;
        }

        try {
            const queryParams = new URLSearchParams({
                containerName: this.props.selectedContainer,
                podName: this.props.podName,
                namespace: this.props.namespace,
                path: node.path || "",
                token: localStorage.getItem('token') || "",
                type: type || ""
            }).toString();

            let url = `/k8s/file/download?${queryParams}`;
            url = ProcessK8sUrlWithCluster(url);
            const a = document.createElement('a');
            a.href = url;
            a.click();
            message.success('文件正在下载...');
        } catch (e) {
            message.error('下载失败，请重试');
        }
    }

    async handleUpload(node: FileNode, onUploadSuccess: () => void) {
        if (!node.isDir) {
            message.error('只能在目录下上传文件');
            return;
        }

        const uploadInput = document.createElement('input');
        uploadInput.type = 'file';
        uploadInput.multiple = true; // Enable multiple file selection
        uploadInput.onchange = async (e) => {
            const files = (e.target as HTMLInputElement).files;
            if (!files || files.length === 0) return;

            const fileArray = Array.from(files);
            const totalFiles = fileArray.length;

            // Show batch upload progress for multiple files
            if (totalFiles > 1) {
                await this.handleBatchUpload(fileArray, node, onUploadSuccess);
            } else {
                await this.handleSingleUpload(fileArray[0], node, onUploadSuccess);
            }
        };
        uploadInput.click();
    }

    // Handle single file upload (existing logic)
    async handleSingleUpload(file: File, node: FileNode, onUploadSuccess: () => void) {
        const formData = new FormData();
        formData.append('file', file);
        formData.append('containerName', this.props.selectedContainer);
        formData.append('podName', this.props.podName);
        formData.append('namespace', this.props.namespace);
        formData.append('isDir', String(node.isDir));
        formData.append('path', String(node.path));
        formData.append('fileName', file.name);

        try {
            const url = ProcessK8sUrlWithCluster('/k8s/file/upload');
            const response = await fetch(url, {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${localStorage.getItem('token')}`
                },
                body: formData
            });
            const result = await response.json();
            if (result.data?.file?.status === 'done') {
                message.success('上传成功');
                onUploadSuccess();
            } else {
                message.error(result.data?.file?.error || '上传失败');
            }
        } catch (error) {
            message.error('上传失败');
        }
    }

    // Handle batch file upload with progress tracking
    async handleBatchUpload(files: File[], node: FileNode, onUploadSuccess: () => void) {
        const totalFiles = files.length;
        let successCount = 0;
        let failedCount = 0;

        // Initialize progress tracking
        this.batchUploadProgress = {
            total: totalFiles,
            completed: 0,
            failed: 0,
            current: '',
            isVisible: true
        };

        // Show initial progress message
        message.info(`开始批量上传 ${totalFiles} 个文件...`);

        // Create progress modal
        const progressModal = Modal.info({
            title: '批量上传进度',
            content: this.renderProgressContent(),
            okText: '关闭',
            onOk: () => {
                this.batchUploadProgress.isVisible = false;
            },
            afterClose: () => {
                this.batchUploadProgress.isVisible = false;
            }
        });

        try {
            // Use the new batch upload endpoint
            const result = await this.uploadFilesBatch(files, node);
            
            successCount = result.success_count;
            failedCount = result.failure_count;
            
            // Update progress
            this.batchUploadProgress.completed = successCount;
            this.batchUploadProgress.failed = failedCount;
            this.batchUploadProgress.current = '完成';

            // Update modal content
            progressModal.update({
                content: this.renderProgressContent()
            });

            // Show final results
            if (failedCount === 0) {
                message.success(`成功上传 ${successCount} 个文件`);
            } else if (successCount === 0) {
                message.error(`上传失败：所有 ${failedCount} 个文件都失败了`);
            } else {
                message.warning(`上传完成：${successCount} 个成功，${failedCount} 个失败`);
            }

            // Refresh the file tree if at least one file was uploaded successfully
            if (successCount > 0) {
                onUploadSuccess();
            }

        } catch (error) {
            message.error('批量上传失败');
            console.error('Batch upload error:', error);
        } finally {
            // Close progress modal after a delay
            setTimeout(() => {
                progressModal.destroy();
                this.batchUploadProgress.isVisible = false;
            }, 2000);
        }
    }

    // Upload files using the batch endpoint
    async uploadFilesBatch(files: File[], node: FileNode) {
        const formData = new FormData();
        
        // Add metadata
        formData.append('containerName', this.props.selectedContainer);
        formData.append('podName', this.props.podName);
        formData.append('namespace', this.props.namespace);
        formData.append('path', String(node.path));

        // Add all files
        files.forEach(file => {
            formData.append('files', file);
        });

        const url = ProcessK8sUrlWithCluster('/k8s/file/batch-upload');
        const response = await fetch(url, {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${localStorage.getItem('token')}`
            },
            body: formData
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const result = await response.json();
        return result.data;
    }

    // Render progress content for the modal
    renderProgressContent() {
        const { total, completed, failed, current } = this.batchUploadProgress;
        const progressPercent = total > 0 ? Math.round(((completed + failed) / total) * 100) : 0;

        return (
            <div style={{ padding: '20px' }}>
                <div style={{ marginBottom: '16px' }}>
                    <div style={{ marginBottom: '8px' }}>
                        <strong>总文件数：</strong> {total}
                    </div>
                    <div style={{ marginBottom: '8px' }}>
                        <strong>已完成：</strong> {completed}
                    </div>
                    <div style={{ marginBottom: '8px' }}>
                        <strong>失败：</strong> {failed}
                    </div>
                    <div style={{ marginBottom: '16px' }}>
                        <strong>当前状态：</strong> {current || '准备中...'}
                    </div>
                </div>
                
                <Progress 
                    percent={progressPercent} 
                    status={failed > 0 ? 'exception' : 'active'}
                    strokeColor={{
                        '0%': '#108ee9',
                        '100%': '#87d068',
                    }}
                />
                
                <div style={{ marginTop: '16px', fontSize: '12px', color: '#666' }}>
                    批量上传支持最多50个文件，大文件可能需要更长时间
                </div>
            </div>
        );
    }

    async fetchData(path: string = '/', isDir: boolean): Promise<FileNode[]> {
        try {
            const response = await fetcher({
                url: `/k8s/file/list?path=${encodeURIComponent(path)}`,
                method: 'post',
                data: {
                    containerName: this.props.selectedContainer,
                    podName: this.props.podName,
                    namespace: this.props.namespace,
                    isDir: isDir,
                    path: path
                }
            });

            //@ts-ignore
            const rows = response.data?.data?.rows || [];
            return rows.map((item: any): FileNode => ({
                name: item.name || '',
                type: item.type || '',
                permissions: item.permissions || '',
                owner: item.owner || '',
                group: item.group || '',
                size: item.size || 0,
                modTime: item.modTime || '',
                path: item.path || '',
                isDir: item.isDir || false,
                isLeaf: !item.isDir,
                title: item.name,
                key: Math.random().toString(36).substring(2, 15) + Math.random().toString(36).substring(2, 15),
            }));
        } catch (error) {
            console.error('Failed to fetch file tree:', error);
            return [];
        }
    }
}