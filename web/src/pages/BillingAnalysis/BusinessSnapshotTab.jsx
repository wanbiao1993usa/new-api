import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { VChart } from '@visactor/react-vchart';
import { initVChartSemiTheme } from '@visactor/vchart-semi-theme';
import {
  Button,
  Card,
  DatePicker,
  Empty,
  Spin,
  Table,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import { CalendarClock, RefreshCcw, Users, Wallet } from 'lucide-react';
import {
  API,
  renderNumber,
  renderQuota,
  showError,
  timestamp2string,
} from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { DATE_RANGE_PRESETS } from '../../constants/console.constants';
import { CHART_CONFIG } from '../../constants/dashboard.constants';

const { Text } = Typography;

const DEFAULT_DAYS = 30;
const DEFAULT_RANGE = [
  timestamp2string(Math.floor(Date.now() / 1000) - 29 * 24 * 60 * 60),
  timestamp2string(Math.floor(Date.now() / 1000)),
];

const emptySnapshot = {
  summary: {
    topup_paid_users_count: 0,
    topup_users_with_balance_count: 0,
    topup_current_balance_sum: 0,
  },
  daily: [],
  days: DEFAULT_DAYS,
  generated_at: 0,
};

const SnapshotCard = ({ icon: Icon, label, value, accentClassName, hint }) => (
  <Card className='!rounded-lg shadow-sm' bodyStyle={{ padding: 16 }}>
    <div className='flex items-center justify-between gap-3 min-w-0'>
      <div className='min-w-0'>
        <Text type='secondary' size='small'>
          {label}
        </Text>
        <div className='mt-2 text-xl font-semibold leading-tight truncate'>
          {value}
        </div>
        {hint ? (
          <Text type='tertiary' size='small' className='mt-1 block'>
            {hint}
          </Text>
        ) : null}
      </div>
      <div
        className={`h-9 w-9 rounded-lg flex items-center justify-center flex-shrink-0 ${accentClassName}`}
      >
        <Icon size={18} strokeWidth={2} />
      </div>
    </div>
  </Card>
);

const percentText = (value) => `${((value || 0) * 100).toFixed(2)}%`;
const percentValueText = (value) => `${(Number(value) || 0).toFixed(2)}%`;

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

const BusinessSnapshotTab = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [snapshot, setSnapshot] = useState(emptySnapshot);
  const [dateRange, setDateRange] = useState(DEFAULT_RANGE);

  useEffect(() => {
    initVChartSemiTheme({
      isWatchingThemeSwitch: true,
    });
  }, []);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const startTimestamp = toTimestamp(dateRange?.[0]);
      const endTimestamp = toTimestamp(dateRange?.[1]);
      const res = await API.get('/api/billing/analysis/snapshot', {
        params: {
          start_timestamp: startTimestamp,
          end_timestamp: endTimestamp,
        },
        disableDuplicate: true,
      });
      const { success, message, data } = res.data;
      if (success) {
        setSnapshot({
          ...emptySnapshot,
          ...(data || {}),
          summary: {
            ...emptySnapshot.summary,
            ...(data?.summary || {}),
          },
          daily: Array.isArray(data?.daily) ? data.daily : [],
        });
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  }, [dateRange]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const summary = snapshot.summary || emptySnapshot.summary;
  const cards = useMemo(
    () => [
      {
        key: 'topup-paid-users',
        label: t('成功充值用户数'),
        value: renderNumber(summary.topup_paid_users_count || 0),
        hint: t('按成功充值订单去重用户'),
        icon: Users,
        accentClassName: 'bg-teal-100 text-teal-700',
      },
      {
        key: 'topup-users',
        label: t('充值过且当前仍有余额的用户数'),
        value: renderNumber(summary.topup_users_with_balance_count || 0),
        hint: t('按当前账户总余额大于 0 统计'),
        icon: Users,
        accentClassName: 'bg-emerald-100 text-emerald-700',
      },
      {
        key: 'topup-balance',
        label: t('充值用户当前账户余额总额'),
        value: renderQuota(summary.topup_current_balance_sum || 0),
        hint: t('直接统计当前账户总余额'),
        icon: Wallet,
        accentClassName: 'bg-amber-100 text-amber-700',
      },
    ],
    [summary, t],
  );

  const countTrendValues = useMemo(
    () =>
      (snapshot.daily || []).flatMap((row) => [
        {
          date: row.date,
          metric: t('累计总用户数'),
          value: Number(row.total_user_count || 0),
        },
        {
          date: row.date,
          metric: t('使用用户数'),
          value: Number(row.used_user_count || 0),
        },
        {
          date: row.date,
          metric: t('新增用户数'),
          value: Number(row.new_user_count || 0),
        },
      ]),
    [snapshot.daily, t],
  );

  const rateTrendValues = useMemo(
    () =>
      (snapshot.daily || []).flatMap((row) => [
        {
          date: row.date,
          metric: t('使用率'),
          value: Number(((row.used_user_rate || 0) * 100).toFixed(2)),
        },
        {
          date: row.date,
          metric: t('新增率'),
          value: Number(((row.new_user_rate || 0) * 100).toFixed(2)),
        },
      ]),
    [snapshot.daily, t],
  );

  const countTrendSpec = useMemo(
    () => ({
      type: 'line',
      data: [
        {
          id: 'businessSnapshotCountTrend',
          values: countTrendValues,
        },
      ],
      xField: 'date',
      yField: 'value',
      seriesField: 'metric',
      padding: {
        top: 12,
        right: 16,
        bottom: 12,
        left: 8,
      },
      title: {
        visible: true,
        text: t('用户规模趋势'),
        subtext: t('展示累计总用户、使用用户和新增用户变化'),
      },
      legends: {
        visible: true,
        orient: 'top',
      },
      point: {
        visible: true,
        style: {
          size: 5,
        },
      },
      line: {
        style: {
          lineWidth: 2,
        },
      },
      axes: [
        {
          orient: 'bottom',
          label: {
            autoRotate: true,
          },
        },
        {
          orient: 'left',
          label: {
            formatMethod: (value) => renderNumber(Number(value || 0)),
          },
        },
      ],
      tooltip: {
        dimension: {
          updateContent: (items) =>
            (items || []).map((item) => ({
              ...item,
              value: renderNumber(Number(item?.datum?.value || 0)),
            })),
        },
      },
      color: {
        specified: {
          [t('累计总用户数')]: '#0f766e',
          [t('使用用户数')]: '#2563eb',
          [t('新增用户数')]: '#f59e0b',
        },
      },
    }),
    [countTrendValues, t],
  );

  const rateTrendSpec = useMemo(
    () => ({
      type: 'area',
      data: [
        {
          id: 'businessSnapshotRateTrend',
          values: rateTrendValues,
        },
      ],
      xField: 'date',
      yField: 'value',
      seriesField: 'metric',
      padding: {
        top: 12,
        right: 16,
        bottom: 12,
        left: 8,
      },
      title: {
        visible: true,
        text: t('用户活跃效率趋势'),
        subtext: t('展示使用率和新增率变化'),
      },
      legends: {
        visible: true,
        orient: 'top',
      },
      point: {
        visible: true,
        style: {
          size: 5,
        },
      },
      area: {
        style: {
          fillOpacity: 0.18,
        },
      },
      line: {
        style: {
          lineWidth: 2,
        },
      },
      axes: [
        {
          orient: 'bottom',
          label: {
            autoRotate: true,
          },
        },
        {
          orient: 'left',
          label: {
            formatMethod: (value) => percentValueText(value),
          },
        },
      ],
      tooltip: {
        dimension: {
          updateContent: (items) =>
            (items || []).map((item) => ({
              ...item,
              value: percentValueText(item?.datum?.value || 0),
            })),
        },
      },
      color: {
        specified: {
          [t('使用率')]: '#7c3aed',
          [t('新增率')]: '#ec4899',
        },
      },
    }),
    [rateTrendValues, t],
  );

  const columns = useMemo(
    () => [
      {
        title: t('日期'),
        dataIndex: 'date',
        key: 'date',
        width: 140,
        fixed: isMobile ? undefined : 'left',
      },
      {
        title: t('使用用户数'),
        dataIndex: 'used_user_count',
        key: 'used_user_count',
        align: 'right',
        width: 120,
        sorter: (a, b) => (a.used_user_count || 0) - (b.used_user_count || 0),
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('新增用户数'),
        dataIndex: 'new_user_count',
        key: 'new_user_count',
        align: 'right',
        width: 120,
        sorter: (a, b) => (a.new_user_count || 0) - (b.new_user_count || 0),
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('累计总用户数'),
        dataIndex: 'total_user_count',
        key: 'total_user_count',
        align: 'right',
        width: 130,
        sorter: (a, b) => (a.total_user_count || 0) - (b.total_user_count || 0),
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('使用率'),
        dataIndex: 'used_user_rate',
        key: 'used_user_rate',
        align: 'right',
        width: 120,
        sorter: (a, b) => (a.used_user_rate || 0) - (b.used_user_rate || 0),
        render: (value) => percentText(value),
      },
      {
        title: t('新增率'),
        dataIndex: 'new_user_rate',
        key: 'new_user_rate',
        align: 'right',
        width: 120,
        sorter: (a, b) => (a.new_user_rate || 0) - (b.new_user_rate || 0),
        render: (value) => percentText(value),
      },
    ],
    [isMobile, t],
  );

  return (
    <div className='space-y-4'>
      <div className='flex items-center justify-between gap-3'>
        <div className='min-w-0'>
          <Text type='secondary'>{t('当前运营快照 + 所选区间趋势')}</Text>
          <div className='mt-2'>
            <DatePicker
              value={dateRange}
              className='w-full sm:w-[420px]'
              type='dateTimeRange'
              placeholder={[t('开始时间'), t('结束时间')]}
              showClear
              pure
              size='small'
              onChange={(value) => {
                if (Array.isArray(value) && value.length === 2) {
                  setDateRange(value);
                } else {
                  setDateRange(DEFAULT_RANGE);
                }
              }}
              presets={DATE_RANGE_PRESETS.map((preset) => ({
                text: t(preset.text),
                start: preset.start(),
                end: preset.end(),
              }))}
            />
          </div>
        </div>
        <Button
          type='tertiary'
          icon={<RefreshCcw size={15} />}
          onClick={refresh}
          loading={loading}
          size='small'
        >
          {t('刷新')}
        </Button>
      </div>

      <Spin spinning={loading}>
        <div className='grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-3'>
          {cards.map((card) => (
            <SnapshotCard key={card.key} {...card} />
          ))}
        </div>

        <div className='grid grid-cols-1 xl:grid-cols-2 gap-3'>
          <Card className='!rounded-lg shadow-sm' bodyStyle={{ padding: 12 }}>
            <div className='h-[320px]'>
              {countTrendValues.length > 0 ? (
                <VChart spec={countTrendSpec} option={CHART_CONFIG} />
              ) : (
                <div className='h-full flex items-center justify-center'>
                  <Empty description={t('暂无数据')} />
                </div>
              )}
            </div>
          </Card>
          <Card className='!rounded-lg shadow-sm' bodyStyle={{ padding: 12 }}>
            <div className='h-[320px]'>
              {rateTrendValues.length > 0 ? (
                <VChart spec={rateTrendSpec} option={CHART_CONFIG} />
              ) : (
                <div className='h-full flex items-center justify-center'>
                  <Empty description={t('暂无数据')} />
                </div>
              )}
            </div>
          </Card>
        </div>

        <Card className='!rounded-lg shadow-sm'>
          <div className='mb-3 flex items-center gap-2'>
            <CalendarClock size={16} className='text-slate-500' />
            <Tooltip
              content={t(
                '使用率 = 当天使用用户数 / 当天累计总用户数；新增率 = 当天新增用户数 / 当天累计总用户数',
              )}
            >
              <Text>{t('所选时间范围按天趋势')}</Text>
            </Tooltip>
          </div>
          <Table
            columns={columns}
            dataSource={snapshot.daily || []}
            rowKey={(record) => record.date}
            size='small'
            pagination={false}
            scroll={{ x: 'max-content' }}
            empty={<Empty description={t('暂无数据')} />}
          />
        </Card>
      </Spin>
    </div>
  );
};

export default BusinessSnapshotTab;
