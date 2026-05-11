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
  Spin,
  Table,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  CalendarClock,
  Eye,
  Layers3,
  ReceiptText,
  Users,
  Wallet,
} from 'lucide-react';
import { API, renderNumber, renderQuota, showError } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import PlanUserUsageModal from '../../components/table/subscriptions/modals/PlanUserUsageModal';

const { Text } = Typography;

const emptySummary = {
  plan_count: 0,
  active_user_count: 0,
  active_subscription_count: 0,
  historical_used_total: 0,
  current_used_total: 0,
  current_remaining_total: 0,
  unlimited_active_subscription_count: 0,
};

const emptyData = {
  summary: emptySummary,
  plans: [],
};

const StatCard = ({ icon: Icon, label, value, accentClassName, hint }) => (
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

const PlanNameCell = ({ title, subtitle }) => (
  <Tooltip content={subtitle ? `${title} · ${subtitle}` : title || '-'}>
    <div className='min-w-0'>
      <div className='font-medium truncate'>{title || '-'}</div>
      {subtitle ? (
        <Text type='tertiary' size='small' className='truncate block'>
          {subtitle}
        </Text>
      ) : null}
    </div>
  </Tooltip>
);

const normalizeData = (data) => ({
  ...emptyData,
  ...(data || {}),
  summary: {
    ...emptySummary,
    ...(data?.summary || {}),
  },
  plans: Array.isArray(data?.plans) ? data.plans : [],
});

const SubscriptionAnalysisTab = ({ params }) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [analysis, setAnalysis] = useState(emptyData);
  const [selectedPlan, setSelectedPlan] = useState(null);
  const [usageModalVisible, setUsageModalVisible] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/billing/analysis/subscription', {
        params,
        disableDuplicate: true,
      });
      const { success, message, data } = res.data;
      if (success) {
        setAnalysis(normalizeData(data));
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  }, [params]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const openPlanUsage = useCallback((planRow) => {
    setSelectedPlan({
      plan: {
        id: Number(planRow?.plan_id || 0),
        title: planRow?.title || '',
        subtitle: planRow?.subtitle || '',
      },
    });
    setUsageModalVisible(true);
  }, []);

  const summary = analysis.summary || emptySummary;
  const cards = useMemo(
    () => [
      {
        key: 'plans',
        label: t('订阅套餐数'),
        value: renderNumber(summary.plan_count || 0),
        hint: t('按套餐维度汇总'),
        icon: Layers3,
        accentClassName: 'bg-slate-100 text-slate-700',
      },
      {
        key: 'users',
        label: t('当前使用人数'),
        value: renderNumber(summary.active_user_count || 0),
        hint: t('按当前生效订阅去重用户'),
        icon: Users,
        accentClassName: 'bg-sky-100 text-sky-700',
      },
      {
        key: 'subs',
        label: t('当前生效订阅数'),
        value: renderNumber(summary.active_subscription_count || 0),
        hint: t('当前仍在有效期内的订阅实例'),
        icon: CalendarClock,
        accentClassName: 'bg-cyan-100 text-cyan-700',
      },
      {
        key: 'historical',
        label: t('历史已用总额'),
        value: renderQuota(summary.historical_used_total || 0),
        hint: t('按消费日志累计订阅抵扣'),
        icon: ReceiptText,
        accentClassName: 'bg-emerald-100 text-emerald-700',
      },
      {
        key: 'current-used',
        label: t('当前已用总额'),
        value: renderQuota(summary.current_used_total || 0),
        hint: t('按当前生效订阅快照统计'),
        icon: ReceiptText,
        accentClassName: 'bg-amber-100 text-amber-700',
      },
      {
        key: 'remaining',
        label: t('当前剩余总额'),
        value: renderQuota(summary.current_remaining_total || 0),
        hint:
          Number(summary.unlimited_active_subscription_count || 0) > 0
            ? `${t('不含不限额订阅')} · ${t('不限额订阅数')} ${renderNumber(summary.unlimited_active_subscription_count || 0)}`
            : t('不含不限额订阅'),
        icon: Wallet,
        accentClassName: 'bg-indigo-100 text-indigo-700',
      },
    ],
    [summary, t],
  );

  const columns = useMemo(
    () => [
      {
        title: t('订阅套餐'),
        dataIndex: 'title',
        key: 'title',
        width: 220,
        fixed: isMobile ? undefined : 'left',
        render: (_, record) => (
          <PlanNameCell title={record?.title} subtitle={record?.subtitle} />
        ),
      },
      {
        title: t('使用人数'),
        dataIndex: 'active_user_count',
        key: 'active_user_count',
        width: 110,
        align: 'right',
        sorter: (a, b) =>
          (a.active_user_count || 0) - (b.active_user_count || 0),
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('生效订阅数'),
        dataIndex: 'active_subscription_count',
        key: 'active_subscription_count',
        width: 120,
        align: 'right',
        sorter: (a, b) =>
          (a.active_subscription_count || 0) -
          (b.active_subscription_count || 0),
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('历史已用总额'),
        dataIndex: 'historical_used_total',
        key: 'historical_used_total',
        width: 140,
        align: 'right',
        sorter: (a, b) =>
          (a.historical_used_total || 0) - (b.historical_used_total || 0),
        render: (value) => renderQuota(value || 0),
      },
      {
        title: t('当前已用总额'),
        dataIndex: 'current_used_total',
        key: 'current_used_total',
        width: 140,
        align: 'right',
        sorter: (a, b) =>
          (a.current_used_total || 0) - (b.current_used_total || 0),
        render: (value) => renderQuota(value || 0),
      },
      {
        title: t('当前剩余总额'),
        dataIndex: 'current_remaining_total',
        key: 'current_remaining_total',
        width: 150,
        align: 'right',
        sorter: (a, b) =>
          (a.current_remaining_total || 0) - (b.current_remaining_total || 0),
        render: (value, record) => (
          <div className='flex flex-col items-end leading-tight'>
            <span>{renderQuota(value || 0)}</span>
            {Number(record?.unlimited_active_subscription_count || 0) > 0 ? (
              <span className='text-[11px] text-slate-400'>
                {t('不限额')}{' '}
                {renderNumber(record?.unlimited_active_subscription_count || 0)}
              </span>
            ) : null}
          </div>
        ),
      },
      {
        title: t('操作'),
        key: 'actions',
        width: 120,
        fixed: isMobile ? undefined : 'right',
        render: (_, record) => (
          <Button
            type='tertiary'
            size='small'
            icon={<Eye size={15} />}
            onClick={() => openPlanUsage(record)}
          >
            {t('查看明细')}
          </Button>
        ),
      },
    ],
    [isMobile, openPlanUsage, t],
  );

  return (
    <>
      <Spin spinning={loading}>
        <div className='grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-6 gap-3'>
          {cards.map((card) => (
            <StatCard key={card.key} {...card} />
          ))}
        </div>

        <Card className='!rounded-lg shadow-sm mt-4' bodyStyle={{ padding: 0 }}>
          <Table
            columns={columns}
            dataSource={analysis.plans}
            rowKey={(record) => `subscription-plan-${record.plan_id}`}
            size='small'
            pagination={{
              pageSize: 10,
              showSizeChanger: true,
              pageSizeOptions: [10, 20, 50],
            }}
            scroll={{ x: 'max-content' }}
            empty={<Empty description={t('搜索无结果')} />}
          />
        </Card>
      </Spin>

      <PlanUserUsageModal
        visible={usageModalVisible}
        onCancel={() => setUsageModalVisible(false)}
        planRecord={selectedPlan}
        t={t}
      />
    </>
  );
};

export default SubscriptionAnalysisTab;
