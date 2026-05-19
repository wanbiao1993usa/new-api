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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Empty,
  Form,
  Spin,
  Table,
  Tabs,
  TabPane,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  CalendarClock,
  Gauge,
  Hash,
  ListChecks,
  ReceiptText,
  RotateCcw,
  Search,
  Wallet,
} from 'lucide-react';
import {
  API,
  getTodayStartTimestamp,
  isAdmin,
  renderNumber,
  renderQuota,
  showError,
  timestamp2string,
} from '../../helpers';
import { DATE_RANGE_PRESETS } from '../../constants/console.constants';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import BusinessSnapshotTab from './BusinessSnapshotTab';
import SubscriptionAnalysisTab from './SubscriptionAnalysisTab';

const { Text, Title } = Typography;

const emptyTokenMetrics = {
  prompt_tokens: 0,
  completion_tokens: 0,
  cache_read_tokens: 0,
  cache_write_tokens: 0,
  cache_tokens: 0,
  total_tokens_with_cache: 0,
  prompt_share: 0,
  completion_share: 0,
  cache_share: 0,
  avg_prompt_tokens_per_request: 0,
  avg_completion_tokens_per_request: 0,
  avg_cache_tokens_per_request: 0,
};

const emptySummary = {
  total_quota: 0,
  original_total_quota: 0,
  wallet_quota: 0,
  wallet_multiplier_overview: [],
  subscription_quota: 0,
  subscription_multiplier_overview: [],
  multiplier_overview: [],
  token_count: 0,
  request_count: 0,
  effective_quota_per_1k_tokens: 0,
  token_metrics: emptyTokenMetrics,
};

const emptyAnalysis = {
  summary: emptySummary,
  users: [],
  tokens: [],
  models: [],
  channels: [],
  groups: [],
};

const compactBillingOverviewLabel = (label) => {
  if (!label || typeof label !== 'string') {
    return '-';
  }

  const parts = label.split(' / ');
  if (parts[0] !== '阶梯计费') {
    return label;
  }

  const ratioPart = parts.find(
    (part) => part.startsWith('分组倍率 ') || part.startsWith('专属倍率 '),
  );
  const isUserRatio = ratioPart?.startsWith('专属倍率 ');
  const ratio = ratioPart
    ?.replace('分组倍率 ', '')
    .replace('专属倍率 ', '')
    .trim();
  const compactRatio = isUserRatio ? `专属 ${ratio}` : ratio;
  const tier = parts.length > 2 ? parts[1]?.trim() : '';
  if (tier && compactRatio) {
    return `${tier} · ${compactRatio}`;
  }
  if (compactRatio) {
    return `阶梯计费 · ${compactRatio}`;
  }
  return label;
};

const getInitialDateRange = () => [
  timestamp2string(getTodayStartTimestamp()),
  timestamp2string(Date.now() / 1000),
];

const toTimestamp = (value) => {
  if (!value) {
    return 0;
  }
  if (value instanceof Date) {
    return Math.floor(value.getTime() / 1000);
  }
  if (typeof value?.valueOf === 'function' && typeof value !== 'string') {
    const timestamp = value.valueOf();
    if (Number.isFinite(timestamp)) {
      return Math.floor(timestamp / 1000);
    }
  }
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? Math.floor(parsed / 1000) : 0;
};

const normalizeAnalysis = (data) => ({
  ...emptyAnalysis,
  ...(data || {}),
  summary: {
    ...emptySummary,
    ...(data?.summary || {}),
    wallet_multiplier_overview: Array.isArray(
      data?.summary?.wallet_multiplier_overview,
    )
      ? data.summary.wallet_multiplier_overview
      : [],
    subscription_multiplier_overview: Array.isArray(
      data?.summary?.subscription_multiplier_overview,
    )
      ? data.summary.subscription_multiplier_overview
      : [],
    multiplier_overview: Array.isArray(data?.summary?.multiplier_overview)
      ? data.summary.multiplier_overview
      : [],
    token_metrics: {
      ...emptyTokenMetrics,
      ...(data?.summary?.token_metrics || {}),
    },
  },
  users: Array.isArray(data?.users) ? data.users : [],
  tokens: Array.isArray(data?.tokens) ? data.tokens : [],
  models: Array.isArray(data?.models) ? data.models : [],
  channels: Array.isArray(data?.channels) ? data.channels : [],
  groups: Array.isArray(data?.groups) ? data.groups : [],
});

const formatPercent = (value) => {
  const num = Number(value);
  if (!Number.isFinite(num) || num <= 0) {
    return '0%';
  }
  const percent = num * 100;
  const digits = percent >= 10 ? 1 : 2;
  return `${percent
    .toFixed(digits)
    .replace(/\.0+$/, '')
    .replace(/(\.\d*[1-9])0+$/, '$1')}%`;
};

const formatAverageTokenValue = (value) => {
  const num = Number(value);
  if (!Number.isFinite(num)) {
    return '0';
  }
  const digits = Number.isInteger(num) || Math.abs(num) >= 100 ? 0 : 1;
  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits: digits,
  }).format(num);
};

const buildBillingAnalysisParams = (values, fallbackValues, isAdminUser) => {
  const safeValues = values || fallbackValues;
  const dateRange = Array.isArray(safeValues?.dateRange)
    ? safeValues.dateRange
    : fallbackValues.dateRange;
  const params = {
    token_name: safeValues?.token_name || '',
    model_name: safeValues?.model_name || '',
    group: safeValues?.group || '',
  };

  const startTimestamp = toTimestamp(dateRange[0]);
  const endTimestamp = toTimestamp(dateRange[1]);
  if (startTimestamp > 0) {
    params.start_timestamp = startTimestamp;
  }
  if (endTimestamp > 0) {
    params.end_timestamp = endTimestamp;
  }
  if (isAdminUser) {
    params.username = safeValues?.username || '';
    params.channel = safeValues?.channel || '';
  }
  return params;
};

const StatCard = ({
  icon: Icon,
  label,
  value,
  accentClassName,
  detailsTitle,
  details,
}) => (
  <Card className='!rounded-lg shadow-sm' bodyStyle={{ padding: 16 }}>
    <div className='flex items-center justify-between gap-3 min-w-0'>
      <div className='min-w-0'>
        <Text type='secondary' size='small'>
          {label}
        </Text>
        <div className='mt-2 text-xl font-semibold leading-tight truncate'>
          {value}
        </div>
      </div>
      <div
        className={`h-9 w-9 rounded-lg flex items-center justify-center flex-shrink-0 ${accentClassName}`}
      >
        <Icon size={18} strokeWidth={2} />
      </div>
    </div>
    {Array.isArray(details) && details.length > 0 && (
      <div className='mt-3 border-t border-slate-100 pt-2 space-y-1'>
        {detailsTitle && (
          <Text type='tertiary' size='small'>
            {detailsTitle}
          </Text>
        )}
        {details.map((detail) => (
          <div
            key={`${label}-${detail.label}`}
            className='flex items-center justify-between gap-2 text-xs text-slate-500'
          >
            <Tooltip content={detail.fullLabel || detail.label}>
              <span
                className='min-w-0 flex-1 truncate'
                title={detail.fullLabel || detail.label}
              >
                {detail.label}
              </span>
            </Tooltip>
            <div className='flex flex-col items-end flex-shrink-0 leading-tight'>
              <span>{detail.value}</span>
              {detail.extra && (
                <span className='text-[11px] text-slate-400'>
                  {detail.extra}
                </span>
              )}
            </div>
          </div>
        ))}
      </div>
    )}
  </Card>
);

const DimensionName = ({ value }) => (
  <Tooltip content={value || '-'}>
    <Text ellipsis={{ showTooltip: false }} style={{ maxWidth: 220 }}>
      {value || '-'}
    </Text>
  </Tooltip>
);

const BillingAnalysis = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const isAdminUser = isAdmin();
  const [formApi, setFormApi] = useState(null);
  const [loading, setLoading] = useState(false);
  const [analysis, setAnalysis] = useState(emptyAnalysis);
  const [adminView, setAdminView] = useState('consume');

  const formInitValues = useMemo(
    () => ({
      username: '',
      token_name: '',
      model_name: '',
      channel: '',
      group: '',
      dateRange: getInitialDateRange(),
    }),
    [],
  );
  const initialQueryParams = useMemo(
    () =>
      buildBillingAnalysisParams(formInitValues, formInitValues, isAdminUser),
    [formInitValues, isAdminUser],
  );
  const [submittedParams, setSubmittedParams] = useState(initialQueryParams);

  const buildParams = useCallback(() => {
    const values = formApi?.getValues?.() || formInitValues;
    return buildBillingAnalysisParams(values, formInitValues, isAdminUser);
  }, [formApi, formInitValues, isAdminUser]);

  const refresh = useCallback(async () => {
    if (isAdminUser && adminView !== 'consume') {
      return;
    }
    setLoading(true);
    try {
      const endpoint = isAdminUser
        ? '/api/billing/analysis/'
        : '/api/billing/analysis/self';
      const res = await API.get(endpoint, {
        params: submittedParams,
        disableDuplicate: true,
      });
      const { success, message, data } = res.data;
      if (success) {
        setAnalysis(normalizeAnalysis(data));
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  }, [adminView, isAdminUser, submittedParams]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const resetFilters = () => {
    if (formApi) {
      formApi.reset();
    }
    setTimeout(() => {
      setSubmittedParams(initialQueryParams);
    }, 0);
  };

  useEffect(() => {
    setSubmittedParams(initialQueryParams);
  }, [initialQueryParams]);

  const submitFilters = () => {
    setSubmittedParams(buildParams());
  };

  const summary = analysis.summary;
  const tokenMetrics = summary.token_metrics || emptyTokenMetrics;
  const buildOverviewDetails = (items) =>
    (Array.isArray(items) ? items : []).slice(0, 3).map((item) => ({
      label: compactBillingOverviewLabel(item?.label || '-'),
      fullLabel: item?.label || '-',
      value: renderQuota(item?.quota || 0),
      extra:
        Number.isFinite(item?.original_quota) && item.original_quota > 0
          ? `${t('原价')} ${renderQuota(item.original_quota)}`
          : '',
    }));
  const buildMetricOverviewDetails = (
    items,
    valueGetter,
    valueFormatter,
    extraGetter,
  ) =>
    (Array.isArray(items) ? [...items] : [])
      .sort((a, b) => (valueGetter(b) || 0) - (valueGetter(a) || 0))
      .slice(0, 3)
      .map((item) => ({
        label: compactBillingOverviewLabel(item?.label || '-'),
        fullLabel: item?.label || '-',
        value: valueFormatter(valueGetter(item) || 0, item),
        extra: extraGetter?.(item) || '',
      }));
  const buildTokenBreakdownDetails = (metrics) => {
    if (!metrics || (metrics.total_tokens_with_cache || 0) <= 0) {
      return [];
    }
    return [
      {
        label: t('输入（不含缓存）'),
        value: renderNumber(metrics.prompt_tokens || 0),
        extra: `${t('占比')} ${formatPercent(metrics.prompt_share)}`,
      },
      {
        label: t('输出'),
        value: renderNumber(metrics.completion_tokens || 0),
        extra: `${t('占比')} ${formatPercent(metrics.completion_share)}`,
      },
      {
        label: t('缓存'),
        value: renderNumber(metrics.cache_tokens || 0),
        extra: `${t('读')} ${renderNumber(metrics.cache_read_tokens || 0)} · ${t('写')} ${renderNumber(metrics.cache_write_tokens || 0)} · ${t('占比')} ${formatPercent(metrics.cache_share)}`,
      },
    ];
  };
  const buildAverageTokenDetails = (metrics, requestCount) => {
    if (!metrics || requestCount <= 0) {
      return [];
    }
    return [
      {
        label: t('平均输入'),
        value: formatAverageTokenValue(metrics.avg_prompt_tokens_per_request),
        extra: `${t('占比')} ${formatPercent(metrics.prompt_share)}`,
      },
      {
        label: t('平均输出'),
        value: formatAverageTokenValue(
          metrics.avg_completion_tokens_per_request,
        ),
        extra: `${t('占比')} ${formatPercent(metrics.completion_share)}`,
      },
      {
        label: t('平均缓存'),
        value: formatAverageTokenValue(metrics.avg_cache_tokens_per_request),
        extra: `${t('占比')} ${formatPercent(metrics.cache_share)}`,
      },
    ];
  };
  const multiplierOverview = summary.multiplier_overview || [];
  const statCards = [
    {
      key: 'total',
      label: t('总消耗额度'),
      value: renderQuota(summary.total_quota),
      icon: ReceiptText,
      accentClassName: 'bg-slate-100 text-slate-700',
      details: [
        {
          label: t('官方原价总额'),
          value: renderQuota(summary.original_total_quota || 0),
        },
      ],
    },
    {
      key: 'wallet',
      label: t('余额消耗'),
      value: renderQuota(summary.wallet_quota),
      icon: Wallet,
      accentClassName: 'bg-emerald-100 text-emerald-700',
      detailsTitle: t('倍率'),
      details: buildOverviewDetails(summary.wallet_multiplier_overview),
    },
    {
      key: 'subscription',
      label: t('订阅抵扣'),
      value: renderQuota(summary.subscription_quota),
      icon: CalendarClock,
      accentClassName: 'bg-sky-100 text-sky-700',
      detailsTitle: t('倍率'),
      details: buildOverviewDetails(summary.subscription_multiplier_overview),
    },
    {
      key: 'tokens',
      label: t('日志 Tokens'),
      value: renderNumber(summary.token_count || 0),
      icon: Hash,
      accentClassName: 'bg-amber-100 text-amber-700',
      detailsTitle: t('结构（含缓存）'),
      details: buildTokenBreakdownDetails(tokenMetrics),
    },
    {
      key: 'requests',
      label: t('消费日志数'),
      value: renderNumber(summary.request_count || 0),
      icon: ListChecks,
      accentClassName: 'bg-rose-100 text-rose-700',
      detailsTitle: t('单次平均'),
      details: buildAverageTokenDetails(
        tokenMetrics,
        summary.request_count || 0,
      ),
    },
    {
      key: 'effective',
      label: t('每 1M Tokens 有效额度'),
      value: renderQuota(summary.effective_quota_per_1k_tokens || 0, 4),
      icon: Gauge,
      accentClassName: 'bg-indigo-100 text-indigo-700',
      detailsTitle: t('倍率'),
      details: buildMetricOverviewDetails(
        multiplierOverview,
        (item) => item?.effective_quota_per_1k_tokens || 0,
        (value) => renderQuota(value, 4),
        (item) => `${t('日志 Tokens')} ${renderNumber(item?.token_count || 0)}`,
      ),
    },
  ];

  const columns = useMemo(
    () => [
      {
        title: t('名称'),
        dataIndex: 'name',
        key: 'name',
        width: 220,
        fixed: isMobile ? undefined : 'left',
        render: (value) => <DimensionName value={value} />,
      },
      {
        title: t('消费日志数'),
        dataIndex: 'request_count',
        key: 'request_count',
        align: 'right',
        width: 120,
        sorter: (a, b) => a.request_count - b.request_count,
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('日志 Tokens'),
        dataIndex: 'token_count',
        key: 'token_count',
        align: 'right',
        width: 130,
        sorter: (a, b) => a.token_count - b.token_count,
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('总消耗额度'),
        dataIndex: 'total_quota',
        key: 'total_quota',
        align: 'right',
        width: 140,
        sorter: (a, b) => a.total_quota - b.total_quota,
        render: (value) => renderQuota(value || 0),
      },
      {
        title: t('余额消耗'),
        dataIndex: 'wallet_quota',
        key: 'wallet_quota',
        align: 'right',
        width: 140,
        sorter: (a, b) => a.wallet_quota - b.wallet_quota,
        render: (value) => renderQuota(value || 0),
      },
      {
        title: t('订阅抵扣'),
        dataIndex: 'subscription_quota',
        key: 'subscription_quota',
        align: 'right',
        width: 140,
        sorter: (a, b) => a.subscription_quota - b.subscription_quota,
        render: (value) => renderQuota(value || 0),
      },
      {
        title: t('每 1M Tokens 有效额度'),
        dataIndex: 'effective_quota_per_1k_tokens',
        key: 'effective_quota_per_1k_tokens',
        align: 'right',
        width: 170,
        sorter: (a, b) =>
          a.effective_quota_per_1k_tokens - b.effective_quota_per_1k_tokens,
        render: (value) => renderQuota(value || 0, 4),
      },
      {
        title: t('最近使用时间'),
        dataIndex: 'last_used_at',
        key: 'last_used_at',
        width: 170,
        sorter: (a, b) => a.last_used_at - b.last_used_at,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
    ],
    [isMobile, t],
  );

  const tabs = useMemo(() => {
    const baseTabs = [
      { itemKey: 'tokens', tab: t('令牌'), data: analysis.tokens },
      { itemKey: 'models', tab: t('模型'), data: analysis.models },
      { itemKey: 'groups', tab: t('分组'), data: analysis.groups },
    ];
    if (!isAdminUser) {
      return baseTabs;
    }
    return [
      { itemKey: 'users', tab: t('用户'), data: analysis.users },
      ...baseTabs,
      { itemKey: 'channels', tab: t('渠道'), data: analysis.channels },
    ];
  }, [analysis, isAdminUser, t]);

  const tablePagination = {
    pageSize: 10,
    showSizeChanger: true,
    pageSizeOptions: [10, 20, 50],
  };

  return (
    <div className='mt-[60px] px-2 pb-6 space-y-4'>
      <div className='flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between'>
        <Title heading={3} className='!mb-0'>
          {t('计费分析')}
        </Title>
      </div>

      {isAdminUser && (
        <Card
          className='!rounded-lg shadow-sm'
          bodyStyle={{ padding: '0 16px' }}
        >
          <Tabs type='line' activeKey={adminView} onChange={setAdminView}>
            <TabPane tab={t('消费分析')} itemKey='consume' />
            <TabPane tab={t('订阅分析')} itemKey='subscription' />
            <TabPane tab={t('运营快照')} itemKey='snapshot' />
          </Tabs>
        </Card>
      )}

      {isAdminUser && adminView === 'snapshot' ? (
        <BusinessSnapshotTab />
      ) : (
        <>
          <Card className='!rounded-lg shadow-sm' bodyStyle={{ padding: 16 }}>
            <Form
              initValues={formInitValues}
              getFormApi={(api) => setFormApi(api)}
              onSubmit={submitFilters}
              allowEmpty={true}
              autoComplete='off'
              layout='vertical'
              trigger='change'
              stopValidateWithError={false}
            >
              <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-2'>
                <div
                  className={
                    isAdminUser && adminView === 'subscription'
                      ? 'md:col-span-2 lg:col-span-2'
                      : 'lg:col-span-2'
                  }
                >
                  <Form.DatePicker
                    field='dateRange'
                    className='w-full'
                    type='dateTimeRange'
                    placeholder={[t('开始时间'), t('结束时间')]}
                    showClear
                    pure
                    size='small'
                    presets={DATE_RANGE_PRESETS.map((preset) => ({
                      text: t(preset.text),
                      start: preset.start(),
                      end: preset.end(),
                    }))}
                  />
                </div>
                {(!isAdminUser || adminView === 'consume') && (
                  <>
                    <Form.Input
                      field='token_name'
                      placeholder={t('令牌名称')}
                      showClear
                      pure
                      size='small'
                    />
                    <Form.Input
                      field='model_name'
                      placeholder={t('模型名称')}
                      showClear
                      pure
                      size='small'
                    />
                    <Form.Input
                      field='group'
                      placeholder={t('分组')}
                      showClear
                      pure
                      size='small'
                    />
                  </>
                )}
                {isAdminUser && (
                  <>
                    <Form.Input
                      field='username'
                      placeholder={t('用户名称')}
                      showClear
                      pure
                      size='small'
                    />
                    {adminView === 'consume' && (
                      <Form.Input
                        field='channel'
                        placeholder={t('渠道 ID')}
                        showClear
                        pure
                        size='small'
                      />
                    )}
                  </>
                )}
              </div>
              <div className='mt-3 flex justify-end gap-2'>
                <Button
                  type='tertiary'
                  htmlType='submit'
                  loading={loading}
                  icon={<Search size={15} />}
                  size='small'
                >
                  {t('查询')}
                </Button>
                <Button
                  type='tertiary'
                  onClick={resetFilters}
                  icon={<RotateCcw size={15} />}
                  size='small'
                >
                  {t('重置')}
                </Button>
              </div>
            </Form>
          </Card>

          {isAdminUser && adminView === 'subscription' ? (
            <SubscriptionAnalysisTab params={submittedParams} />
          ) : (
            <Spin spinning={loading}>
              <div className='grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-6 gap-3'>
                {statCards.map((card) => (
                  <StatCard key={card.key} {...card} />
                ))}
              </div>

              <Card
                className='!rounded-lg shadow-sm mt-4'
                bodyStyle={{ padding: 0 }}
              >
                <Tabs type='line' className='px-4 pt-2'>
                  {tabs.map((tab) => (
                    <TabPane
                      tab={tab.tab}
                      itemKey={tab.itemKey}
                      key={tab.itemKey}
                    >
                      <Table
                        columns={columns}
                        dataSource={tab.data}
                        rowKey={(record) => `${tab.itemKey}-${record.key}`}
                        size='small'
                        pagination={tablePagination}
                        scroll={{ x: 'max-content' }}
                        empty={<Empty description={t('搜索无结果')} />}
                      />
                    </TabPane>
                  ))}
                </Tabs>
              </Card>
            </Spin>
          )}
        </>
      )}
    </div>
  );
};

export default BillingAnalysis;
