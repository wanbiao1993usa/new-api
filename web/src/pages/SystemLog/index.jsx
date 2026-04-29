/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Button,
  Card,
  Empty,
  Input,
  InputNumber,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  Copy,
  Download,
  FileText,
  RefreshCw,
  Search,
  Terminal,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, copy, showError, showSuccess } from '../../helpers';

const { Text, Title } = Typography;

const defaultTailLines = 500;

const formatBytes = (value) => {
  const bytes = Number(value);
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let size = bytes;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  return `${size.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
};

const formatTime = (value) => {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleString();
};

const SystemLog = () => {
  const { t } = useTranslation();
  const [files, setFiles] = useState([]);
  const [selectedFile, setSelectedFile] = useState('');
  const [tailLines, setTailLines] = useState(defaultTailLines);
  const [keyword, setKeyword] = useState('');
  const [logData, setLogData] = useState(null);
  const [filesLoading, setFilesLoading] = useState(false);
  const [logsLoading, setLogsLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [lastLoadedAt, setLastLoadedAt] = useState(null);
  const logPanelRef = useRef(null);

  const logText = useMemo(() => {
    const lines = logData?.lines || [];
    return lines.join('\n');
  }, [logData]);

  const selectedFileInfo = useMemo(
    () => files.find((item) => item.name === selectedFile),
    [files, selectedFile],
  );

  const fileOptions = useMemo(
    () =>
      files.map((file) => ({
        label: `${file.name} · ${formatBytes(file.size)}`,
        value: file.name,
      })),
    [files],
  );

  const scrollToBottom = () => {
    if (!logPanelRef.current) return;
    logPanelRef.current.scrollTop = logPanelRef.current.scrollHeight;
  };

  const loadLogContent = async (fileName = selectedFile) => {
    setLogsLoading(true);
    try {
      const res = await API.get('/api/system-log/content', {
        params: {
          file: fileName || undefined,
          tail: tailLines,
          keyword: keyword || undefined,
        },
        disableDuplicate: true,
      });
      const { success, message, data } = res.data;
      if (success) {
        setLogData(data);
        if (data?.file?.name) {
          setSelectedFile(data.file.name);
        }
        setLastLoadedAt(new Date());
        setTimeout(scrollToBottom, 50);
      } else {
        showError(t(message));
      }
    } catch (error) {
      showError(t('获取系统日志失败'));
    } finally {
      setLogsLoading(false);
    }
  };

  const loadLogFiles = async () => {
    setFilesLoading(true);
    try {
      const res = await API.get('/api/system-log/files', {
        disableDuplicate: true,
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(t(message));
        return;
      }
      const nextFiles = data?.files || [];
      setFiles(nextFiles);
      const nextSelected =
        selectedFile && nextFiles.some((file) => file.name === selectedFile)
          ? selectedFile
          : nextFiles[0]?.name || '';
      setSelectedFile(nextSelected);
      if (nextSelected) {
        await loadLogContent(nextSelected);
      } else {
        setLogData({ enabled: data?.enabled === true, lines: [] });
      }
    } catch (error) {
      showError(t('获取系统日志文件失败'));
    } finally {
      setFilesLoading(false);
    }
  };

  const handleCopy = async () => {
    if (!logText) return;
    if (await copy(logText)) {
      showSuccess(t('日志已复制到剪贴板'));
    }
  };

  const handleDownload = () => {
    if (!logText) return;
    const blob = new Blob([logText], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = selectedFile || 'system-log.txt';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
    showSuccess(t('日志已下载'));
  };

  useEffect(() => {
    loadLogFiles();
  }, []);

  useEffect(() => {
    if (!autoRefresh) return undefined;
    const timer = setInterval(() => {
      loadLogContent();
    }, 5000);
    return () => clearInterval(timer);
  }, [autoRefresh, selectedFile, tailLines, keyword]);

  return (
    <div className='mt-[60px] px-2'>
      <Card className='!rounded-xl shadow-sm'>
        <div className='flex flex-col gap-4'>
          <div className='flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
            <div className='flex min-w-0 items-center gap-3'>
              <div className='flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-700'>
                <Terminal size={20} />
              </div>
              <div className='min-w-0'>
                <Title heading={4} className='!m-0'>
                  {t('系统日志')}
                </Title>
                <div className='mt-1 flex flex-wrap items-center gap-2 text-xs text-semi-color-text-2'>
                  <Tag color='blue' shape='circle'>
                    {t('管理员可见')}
                  </Tag>
                  <span>{`${t('日志文件')}：${files.length}`}</span>
                  {selectedFileInfo && (
                    <span>{`${formatBytes(selectedFileInfo.size)} · ${formatTime(
                      selectedFileInfo.mod_time,
                    )}`}</span>
                  )}
                </div>
              </div>
            </div>
            <Space wrap>
              <Text size='small' type='secondary'>
                {lastLoadedAt
                  ? `${t('最后刷新')}：${lastLoadedAt.toLocaleTimeString()}`
                  : t('尚未刷新')}
              </Text>
              <Switch
                checked={autoRefresh}
                onChange={setAutoRefresh}
                checkedText={t('自动')}
                uncheckedText={t('手动')}
              />
            </Space>
          </div>

          <div className='grid grid-cols-1 gap-3 lg:grid-cols-[minmax(220px,1fr)_160px_minmax(180px,260px)_auto]'>
            <Select
              prefix={<FileText size={16} />}
              placeholder={t('选择日志文件')}
              optionList={fileOptions}
              value={selectedFile}
              loading={filesLoading}
              disabled={files.length === 0}
              filter
              style={{ width: '100%' }}
              onChange={(value) => {
                const nextFile = value || '';
                setSelectedFile(nextFile);
                if (nextFile) {
                  loadLogContent(nextFile);
                }
              }}
            />
            <InputNumber
              min={1}
              max={5000}
              value={tailLines}
              prefix={t('尾部')}
              suffix={t('行')}
              onChange={(value) => setTailLines(Number(value) || 1)}
            />
            <Input
              prefix={<Search size={16} />}
              placeholder={t('关键词过滤')}
              value={keyword}
              showClear
              onChange={setKeyword}
              onEnterPress={() => loadLogContent()}
            />
            <Space wrap>
              <Button
                type='primary'
                theme='solid'
                icon={<RefreshCw size={16} />}
                loading={logsLoading || filesLoading}
                onClick={() => loadLogContent()}
              >
                {t('刷新')}
              </Button>
              <Button
                type='tertiary'
                icon={<Copy size={16} />}
                disabled={!logText}
                onClick={handleCopy}
              >
                {t('复制')}
              </Button>
              <Button
                type='tertiary'
                icon={<Download size={16} />}
                disabled={!logText}
                onClick={handleDownload}
              >
                {t('下载')}
              </Button>
            </Space>
          </div>

          {logData?.truncated && (
            <div className='rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700'>
              {logData.read_limit_hit
                ? t('日志较大，已按读取上限截取尾部内容')
                : t('已按尾部行数截取日志内容')}
            </div>
          )}
        </div>
      </Card>

      <Card
        className='!mt-3 !rounded-xl shadow-sm'
        bodyStyle={{ padding: 0, overflow: 'hidden' }}
      >
        <Spin spinning={logsLoading || filesLoading}>
          {!logData?.enabled ? (
            <div className='py-16'>
              <Empty
                title={t('系统日志未启用')}
                description={t('请确认服务启动参数已配置日志目录')}
              />
            </div>
          ) : files.length === 0 ? (
            <div className='py-16'>
              <Empty
                title={t('暂无系统日志')}
                description={t('当前日志目录中没有 oneapi 日志文件')}
              />
            </div>
          ) : logText ? (
            <pre
              ref={logPanelRef}
              className='m-0 max-h-[calc(100vh-310px)] min-h-[460px] overflow-auto bg-slate-950 p-4 font-mono text-xs leading-6 text-slate-100'
            >
              {logText}
            </pre>
          ) : (
            <div className='py-16'>
              <Empty
                title={t('没有匹配的日志')}
                description={t('可以调整关键词或增加尾部行数后刷新')}
              />
            </div>
          )}
        </Spin>
      </Card>
    </div>
  );
};

export default SystemLog;
