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

const { Text, Title } = Typography;

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
  },
  users: Array.isArray(data?.users) ? data.users : [],
  tokens: Array.isArray(data?.tokens) ? data.tokens : [],
  models: Array.isArray(data?.models) ? data.models : [],
  channels: Array.isArray(data?.channels) ? data.channels : [],
  groups: Array.isArray(data?.groups) ? data.groups : [],
});

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

  const buildParams = useCallback(() => {
    const values = formApi?.getValues?.() || formInitValues;
    const dateRange = Array.isArray(values.dateRange)
      ? values.dateRange
      : formInitValues.dateRange;
    const params = {
      token_name: values.token_name || '',
      model_name: values.model_name || '',
      group: values.group || '',
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
      params.username = values.username || '';
      params.channel = values.channel || '';
    }
    return params;
  }, [formApi, formInitValues, isAdminUser]);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const endpoint = isAdminUser
        ? '/api/billing/analysis/'
        : '/api/billing/analysis/self';
      const res = await API.get(endpoint, {
        params: buildParams(),
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
  }, [buildParams, isAdminUser]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const resetFilters = () => {
    if (formApi) {
      formApi.reset();
    }
    setTimeout(() => {
      refresh();
    }, 0);
  };

  const summary = analysis.summary;
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
      detailsTitle: t('倍率'),
      details: buildMetricOverviewDetails(
        multiplierOverview,
        (item) => item?.token_count || 0,
        (value) => renderNumber(value),
        (item) =>
          `${t('消费日志数')} ${renderNumber(item?.request_count || 0)}`,
      ),
    },
    {
      key: 'requests',
      label: t('消费日志数'),
      value: renderNumber(summary.request_count || 0),
      icon: ListChecks,
      accentClassName: 'bg-rose-100 text-rose-700',
      detailsTitle: t('倍率'),
      details: buildMetricOverviewDetails(
        multiplierOverview,
        (item) => item?.request_count || 0,
        (value) => renderNumber(value),
        (item) => `${t('日志 Tokens')} ${renderNumber(item?.token_count || 0)}`,
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
              onSubmit={refresh}
              allowEmpty={true}
              autoComplete='off'
              layout='vertical'
              trigger='change'
              stopValidateWithError={false}
            >
              <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-2'>
                <div className='lg:col-span-2'>
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
                {isAdminUser && (
                  <>
                    <Form.Input
                      field='username'
                      placeholder={t('用户名称')}
                      showClear
                      pure
                      size='small'
                    />
                    <Form.Input
                      field='channel'
                      placeholder={t('渠道 ID')}
                      showClear
                      pure
                      size='small'
                    />
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
        </>
      )}
    </div>
  );
};

export default BillingAnalysis;
