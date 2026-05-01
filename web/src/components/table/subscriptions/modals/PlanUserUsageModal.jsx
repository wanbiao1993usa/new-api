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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Empty,
  Input,
  Progress,
  Select,
  SideSheet,
  Space,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { API, renderQuota, showError } from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import CardTable from '../../../common/ui/CardTable';

const { Text, Title } = Typography;

function formatTs(ts) {
  if (!ts) return '-';
  return new Date(ts * 1000).toLocaleString();
}

function getSubscriptionStatus(sub) {
  const now = Date.now() / 1000;
  const end = sub?.end_time || 0;
  if (sub?.status === 'cancelled') return 'cancelled';
  if (sub?.status === 'active' && (!end || end > now)) return 'active';
  return 'expired';
}

function renderStatusTag(sub, t) {
  const status = getSubscriptionStatus(sub);
  if (status === 'active') {
    return (
      <Tag color='green' shape='circle' size='small'>
        {t('生效')}
      </Tag>
    );
  }
  if (status === 'cancelled') {
    return (
      <Tag color='grey' shape='circle' size='small'>
        {t('已作废')}
      </Tag>
    );
  }
  return (
    <Tag color='grey' shape='circle' size='small'>
      {t('已过期')}
    </Tag>
  );
}

function clampPercent(value) {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(100, Math.round(value)));
}

function compareModelLimitKeys(a, b) {
  if (a === b) return 0;
  if (a === '*') return 1;
  if (b === '*') return -1;
  const aWildcard = a.endsWith('*');
  const bWildcard = b.endsWith('*');
  if (aWildcard !== bWildcard) return aWildcard ? 1 : -1;
  return a.localeCompare(b);
}

function getModelLimitSortRank(modelName) {
  if (modelName === '*') return 2;
  if (modelName.endsWith('*')) return 1;
  return 0;
}

function compareModelLimitEntries(a, b) {
  const rankDiff =
    getModelLimitSortRank(a.modelName) - getModelLimitSortRank(b.modelName);
  if (rankDiff !== 0) return rankDiff;
  if (a.limit !== b.limit) return b.limit - a.limit;
  return compareModelLimitKeys(a.modelName, b.modelName);
}

function getRecordUserId(record) {
  return Number(record?.subscription?.user_id || record?.user?.id || 0);
}

function getHistoricalUserUsed(userId, historicalUsageByUser) {
  if (!historicalUsageByUser || userId <= 0) return 0;
  return Number(
    historicalUsageByUser[userId] || historicalUsageByUser[String(userId)] || 0,
  );
}

function getHistoricalUserCallCount(userId, historicalCallCountByUser) {
  if (!historicalCallCountByUser || userId <= 0) return 0;
  return Number(
    historicalCallCountByUser[userId] ||
      historicalCallCountByUser[String(userId)] ||
      0,
  );
}

function compareSubscriptionUsageRecords(a, b) {
  const amountUsedDiff =
    Number(b?.subscription?.amount_used || 0) -
    Number(a?.subscription?.amount_used || 0);
  if (amountUsedDiff !== 0) return amountUsedDiff;

  const aStatus = getSubscriptionStatus(a?.subscription);
  const bStatus = getSubscriptionStatus(b?.subscription);
  if (aStatus !== bStatus) {
    if (aStatus === 'active') return -1;
    if (bStatus === 'active') return 1;
  }

  const endTimeDiff =
    Number(b?.subscription?.end_time || 0) -
    Number(a?.subscription?.end_time || 0);
  if (endTimeDiff !== 0) return endTimeDiff;

  return Number(b?.subscription?.id || 0) - Number(a?.subscription?.id || 0);
}

function buildUserUsageSortMeta(records, historicalUsageByUser) {
  const meta = new Map();

  (records || []).forEach((record) => {
    const userId = getRecordUserId(record);
    const sub = record?.subscription || {};
    const current = meta.get(userId) || {
      historicalTotalUsed: getHistoricalUserUsed(userId, historicalUsageByUser),
      currentCycleUsed: 0,
      activeCount: 0,
      latestEndTime: 0,
    };

    current.currentCycleUsed += Number(sub.amount_used || 0);
    if (getSubscriptionStatus(sub) === 'active') {
      current.activeCount += 1;
    }
    current.latestEndTime = Math.max(
      current.latestEndTime,
      Number(sub.end_time || 0),
    );
    meta.set(userId, current);
  });

  return meta;
}

function compareUserUsageRecords(a, b, userUsageMeta) {
  const aUserId = getRecordUserId(a);
  const bUserId = getRecordUserId(b);
  const aMeta = userUsageMeta.get(aUserId) || {
    historicalTotalUsed: 0,
    currentCycleUsed: 0,
    activeCount: 0,
    latestEndTime: 0,
  };
  const bMeta = userUsageMeta.get(bUserId) || {
    historicalTotalUsed: 0,
    currentCycleUsed: 0,
    activeCount: 0,
    latestEndTime: 0,
  };

  if (aUserId !== bUserId) {
    const aEffectiveUsed =
      aMeta.historicalTotalUsed > 0
        ? aMeta.historicalTotalUsed
        : aMeta.currentCycleUsed;
    const bEffectiveUsed =
      bMeta.historicalTotalUsed > 0
        ? bMeta.historicalTotalUsed
        : bMeta.currentCycleUsed;
    const totalUsedDiff = bEffectiveUsed - aEffectiveUsed;
    if (totalUsedDiff !== 0) return totalUsedDiff;

    const activeCountDiff = bMeta.activeCount - aMeta.activeCount;
    if (activeCountDiff !== 0) return activeCountDiff;

    const latestEndTimeDiff = bMeta.latestEndTime - aMeta.latestEndTime;
    if (latestEndTimeDiff !== 0) return latestEndTimeDiff;

    return bUserId - aUserId;
  }

  return compareSubscriptionUsageRecords(a, b);
}

function getModelLimitEntries(record) {
  const limits = record?.model_amount_limits;
  if (!limits || typeof limits !== 'object') return [];
  const usages = record?.model_amount_limit_usages || {};
  return Object.entries(limits)
    .map(([modelName, limit]) => ({
      modelName,
      limit: Number(limit || 0),
      used: Number(usages?.[modelName] || 0),
    }))
    .sort(compareModelLimitEntries);
}

function getUsageStroke(remainingPercent) {
  return remainingPercent <= 20
    ? 'var(--semi-color-danger)'
    : remainingPercent <= 50
      ? 'var(--semi-color-warning)'
      : 'var(--semi-color-success)';
}

function buildCurrentSnapshot(records) {
  const allRecords = records || [];
  const activeRecords = (records || []).filter(
    (record) => getSubscriptionStatus(record?.subscription) === 'active',
  );
  const activeUsers = new Set();
  const modelStats = {};

  let totalUsedAll = 0;
  let limitedTotal = 0;
  let limitedUsed = 0;
  let unlimitedUsed = 0;
  let unlimitedSubscriptions = 0;

  allRecords.forEach((record) => {
    totalUsedAll += Number(record?.subscription?.amount_used || 0);
  });

  activeRecords.forEach((record) => {
    const sub = record?.subscription || {};
    if (sub.user_id) {
      activeUsers.add(sub.user_id);
    }

    const total = Number(sub.amount_total || 0);
    const used = Number(sub.amount_used || 0);
    if (total > 0) {
      limitedTotal += total;
      limitedUsed += used;
    } else {
      unlimitedSubscriptions += 1;
      unlimitedUsed += used;
    }

    getModelLimitEntries(record).forEach(({ modelName, limit, used }) => {
      if (!modelStats[modelName]) {
        modelStats[modelName] = {
          modelName,
          limit: 0,
          used: 0,
        };
      }
      modelStats[modelName].limit += limit;
      modelStats[modelName].used += used;
    });
  });

  const modelEntries = Object.values(modelStats).sort(compareModelLimitEntries);

  return {
    activeSubscriptionCount: activeRecords.length,
    activeUserCount: activeUsers.size,
    totalUsedAll,
    limitedTotal,
    limitedUsed,
    unlimitedUsed,
    unlimitedSubscriptions,
    modelEntries,
  };
}

function renderTotalUsage(record, t) {
  const sub = record?.subscription;
  const total = Number(sub?.amount_total || 0);
  const used = Number(sub?.amount_used || 0);
  if (total <= 0) {
    return <Text type='tertiary'>{t('不限')}</Text>;
  }
  const usedPercent = clampPercent((used / total) * 100);
  const remain = Math.max(0, total - used);
  return (
    <div className='min-w-[160px]'>
      <div className='flex items-center justify-between gap-2 text-xs'>
        <span className='text-gray-500'>{t('剩余')}</span>
        <span className='font-medium text-gray-700'>
          {Math.max(0, 100 - usedPercent)}%
        </span>
      </div>
      <Progress
        percent={usedPercent}
        showInfo={false}
        stroke='var(--semi-color-primary)'
        style={{ marginTop: 4, marginBottom: 0 }}
      />
      <div className='mt-1 text-[11px] text-gray-500'>
        {t('已用')} {renderQuota(used)} / {renderQuota(total)}
      </div>
      <div className='text-[11px] text-gray-400'>
        {t('剩余')} {renderQuota(remain)}
      </div>
    </div>
  );
}

function renderModelLimitUsage(record, t) {
  const entries = getModelLimitEntries(record);
  if (entries.length === 0) {
    return <Text type='tertiary'>{t('未配置')}</Text>;
  }

  return (
    <div className='space-y-2 min-w-[280px]'>
      {entries.map(({ modelName, limit, used }) => {
        const label = modelName === '*' ? t('其他模型') : modelName;
        const remain = limit > 0 ? Math.max(0, limit - used) : 0;
        const remainingPercent =
          limit > 0 ? clampPercent((remain / limit) * 100) : 0;
        const usedPercent = limit > 0 ? clampPercent((used / limit) * 100) : 0;
        const stroke = getUsageStroke(remainingPercent);

        return (
          <div key={modelName} className='rounded-md bg-gray-50 px-2 py-1.5'>
            <div className='flex items-center justify-between gap-2 text-xs'>
              <Tooltip content={modelName}>
                <span className='truncate font-medium text-gray-700'>
                  {label}
                </span>
              </Tooltip>
              <span className='shrink-0 text-gray-500'>
                {t('剩余')} {remainingPercent}%
              </span>
            </div>
            <Progress
              percent={usedPercent}
              showInfo={false}
              stroke={stroke}
              aria-label={`${label} usage`}
              style={{ marginTop: 4, marginBottom: 0 }}
            />
            <div className='mt-1 text-[11px] text-gray-500'>
              {t('已用')} {renderQuota(used)} / {renderQuota(limit)}
            </div>
          </div>
        );
      })}
    </div>
  );
}

function renderCurrentSnapshot(snapshot, recordCount, t) {
  const historicalUsedTotal = Math.max(
    Number(snapshot.historicalUsedTotal || 0),
    Number(snapshot.totalUsedAll || 0),
  );
  const activeUsedTotal = snapshot.limitedUsed + snapshot.unlimitedUsed;
  const previousUsedTotal = Math.max(0, historicalUsedTotal - activeUsedTotal);

  return (
    <div className='mb-4 grid gap-3 md:grid-cols-3'>
      <div className='rounded-lg border border-gray-200 bg-white px-3 py-3'>
        <Text type='tertiary'>{t('当前生效')}</Text>
        <div className='mt-2 flex items-end gap-2'>
          <span className='text-2xl font-semibold text-gray-900'>
            {snapshot.activeSubscriptionCount}
          </span>
          <Text type='secondary'>{t('个订阅')}</Text>
        </div>
        <div className='mt-1 text-xs text-gray-500'>
          {t('用户')} {snapshot.activeUserCount}
        </div>
      </div>

      <div className='rounded-lg border border-gray-200 bg-white px-3 py-3'>
        <div className='flex items-center justify-between gap-2'>
          <Text type='tertiary'>{t('累计已用额度')}</Text>
          <Text strong>{renderQuota(historicalUsedTotal)}</Text>
        </div>
        {recordCount === 0 ? (
          <div className='mt-2 text-xs text-gray-500'>{t('暂无订阅记录')}</div>
        ) : (
          <div className='mt-2 space-y-1 text-xs text-gray-500'>
            <div>
              {t('当前生效订阅当前周期已用')} {renderQuota(activeUsedTotal)}
            </div>
            <div className='text-xs text-gray-400'>
              {t('历史累计往期已用')} {renderQuota(previousUsedTotal)}
            </div>
          </div>
        )}
        {snapshot.unlimitedSubscriptions > 0 && (
          <div className='mt-1 text-xs text-gray-400'>
            {snapshot.unlimitedSubscriptions} {t('个不限订阅已用')}{' '}
            {renderQuota(snapshot.unlimitedUsed)}
          </div>
        )}
      </div>

      <div className='rounded-lg border border-gray-200 bg-white px-3 py-3'>
        <div className='flex items-center justify-between gap-2'>
          <Text type='tertiary'>{t('当前模型限额')}</Text>
          <Tag color='cyan' shape='circle' size='small'>
            {snapshot.modelEntries.length}
          </Tag>
        </div>
        {snapshot.modelEntries.length === 0 ? (
          <div className='mt-3 text-xs text-gray-500'>{t('未配置')}</div>
        ) : (
          <div className='mt-2 max-h-[154px] space-y-2 overflow-auto pr-1'>
            {snapshot.modelEntries.map(({ modelName, limit, used }) => {
              const label = modelName === '*' ? t('其他模型') : modelName;
              const remain = limit > 0 ? Math.max(0, limit - used) : 0;
              const remainingPercent =
                limit > 0 ? clampPercent((remain / limit) * 100) : 0;
              const usedPercent =
                limit > 0 ? clampPercent((used / limit) * 100) : 0;
              return (
                <div key={modelName}>
                  <div className='flex items-center justify-between gap-2 text-xs'>
                    <Tooltip content={modelName}>
                      <span className='truncate font-medium text-gray-700'>
                        {label}
                      </span>
                    </Tooltip>
                    <span className='shrink-0 text-gray-500'>
                      {t('剩余')} {remainingPercent}%
                    </span>
                  </div>
                  <Progress
                    percent={usedPercent}
                    showInfo={false}
                    stroke={getUsageStroke(remainingPercent)}
                    style={{ marginTop: 4, marginBottom: 0 }}
                  />
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

const PlanUserUsageModal = ({ visible, onCancel, planRecord, t }) => {
  const isMobile = useIsMobile();
  const plan = planRecord?.plan;
  const [loading, setLoading] = useState(false);
  const [records, setRecords] = useState([]);
  const [historicalUsage, setHistoricalUsage] = useState({
    historical_used_total: 0,
    historical_used_by_user: {},
    historical_call_count_total: 0,
    historical_call_count_by_user: {},
  });
  const [keyword, setKeyword] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [currentPage, setCurrentPage] = useState(1);
  const pageSize = 10;

  const loadUsage = async () => {
    if (!plan?.id) return;
    setLoading(true);
    try {
      const res = await API.get(
        `/api/subscription/admin/plans/${plan.id}/user_subscriptions`,
      );
      if (res.data?.success) {
        const payload = res.data.data;
        if (Array.isArray(payload)) {
          setRecords(payload);
          setHistoricalUsage({
            historical_used_total: 0,
            historical_used_by_user: {},
            historical_call_count_total: 0,
            historical_call_count_by_user: {},
          });
        } else {
          setRecords(payload?.records || []);
          setHistoricalUsage(
            payload?.historical_usage || {
              historical_used_total: 0,
              historical_used_by_user: {},
              historical_call_count_total: 0,
              historical_call_count_by_user: {},
            },
          );
        }
      } else {
        showError(res.data?.message || t('加载失败'));
      }
    } catch {
      showError(t('请求失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (visible) {
      loadUsage();
    } else {
      setKeyword('');
      setStatusFilter('all');
      setCurrentPage(1);
      setRecords([]);
      setHistoricalUsage({
        historical_used_total: 0,
        historical_used_by_user: {},
        historical_call_count_total: 0,
        historical_call_count_by_user: {},
      });
    }
  }, [visible, plan?.id]);

  const statusOptions = useMemo(
    () => [
      { label: t('全部状态'), value: 'all' },
      { label: t('生效'), value: 'active' },
      { label: t('已过期'), value: 'expired' },
      { label: t('已作废'), value: 'cancelled' },
    ],
    [t],
  );

  const filteredRecords = useMemo(() => {
    const q = keyword.trim().toLowerCase();
    const filtered = (records || []).filter((record) => {
      const sub = record?.subscription || {};
      const user = record?.user || {};
      if (
        statusFilter !== 'all' &&
        getSubscriptionStatus(sub) !== statusFilter
      ) {
        return false;
      }
      if (!q) return true;
      return [
        sub.id,
        sub.user_id,
        user.id,
        user.username,
        user.display_name,
        user.email,
        user.group,
      ]
        .filter((item) => item !== undefined && item !== null)
        .join(' ')
        .toLowerCase()
        .includes(q);
    });

    const userUsageMeta = buildUserUsageSortMeta(
      filtered,
      historicalUsage?.historical_used_by_user,
    );
    return filtered.sort((a, b) =>
      compareUserUsageRecords(a, b, userUsageMeta),
    );
  }, [records, keyword, statusFilter, historicalUsage]);

  useEffect(() => {
    setCurrentPage(1);
  }, [keyword, statusFilter, records.length]);

  const paginatedRecords = useMemo(() => {
    const start = Math.max(0, (currentPage - 1) * pageSize);
    return filteredRecords.slice(start, start + pageSize);
  }, [filteredRecords, currentPage]);

  const currentSnapshot = useMemo(
    () => ({
      ...buildCurrentSnapshot(records),
      historicalUsedTotal: historicalUsage?.historical_used_total || 0,
    }),
    [records, historicalUsage],
  );

  const columns = useMemo(
    () => [
      {
        title: t('用户'),
        key: 'user',
        width: 220,
        render: (_, record) => {
          const user = record?.user || {};
          const sub = record?.subscription || {};
          return (
            <div className='min-w-[180px]'>
              <div className='font-medium truncate'>
                {user.display_name || user.username || `#${sub.user_id}`}
              </div>
              <div className='text-xs text-gray-500'>
                ID: {sub.user_id}
                {user.username ? ` · ${user.username}` : ''}
              </div>
              <div className='text-xs text-gray-400'>
                {t('历史总调用次数')}{' '}
                {getHistoricalUserCallCount(
                  getRecordUserId(record),
                  historicalUsage?.historical_call_count_by_user,
                )}
              </div>
              {user.email && (
                <div className='text-xs text-gray-400 truncate'>
                  {user.email}
                </div>
              )}
              {user.group && (
                <Tag size='small' shape='circle' color='blue'>
                  {user.group}
                </Tag>
              )}
            </div>
          );
        },
      },
      {
        title: t('历史消费总额'),
        key: 'historical_total_used',
        width: 160,
        render: (_, record) => {
          const historicalUsed = getHistoricalUserUsed(
            getRecordUserId(record),
            historicalUsage?.historical_used_by_user,
          );
          return (
            <div className='min-w-[140px]'>
              <div className='font-medium text-gray-900'>
                {renderQuota(historicalUsed)}
              </div>
              <div className='text-xs text-gray-400'>
                {t('含已过期订阅')}
              </div>
            </div>
          );
        },
      },
      {
        title: t('订阅'),
        key: 'subscription',
        width: 100,
        render: (_, record) => {
          const sub = record?.subscription || {};
          return (
            <div className='text-xs text-gray-600'>
              <div>#{sub.id}</div>
              <div>{sub.source || '-'}</div>
            </div>
          );
        },
      },
      {
        title: t('状态'),
        key: 'status',
        width: 90,
        render: (_, record) => renderStatusTag(record?.subscription, t),
      },
      {
        title: t('有效期'),
        key: 'validity',
        width: 210,
        render: (_, record) => {
          const sub = record?.subscription || {};
          return (
            <div className='text-xs text-gray-600'>
              <div>
                {t('开始')}: {formatTs(sub.start_time)}
              </div>
              <div>
                {t('结束')}: {formatTs(sub.end_time)}
              </div>
              <div>
                {t('下次重置')}: {formatTs(sub.next_reset_time)}
              </div>
            </div>
          );
        },
      },
      {
        title: t('总额度'),
        key: 'total',
        width: 190,
        render: (_, record) => renderTotalUsage(record, t),
      },
      {
        title: t('模型限额'),
        key: 'model_limits',
        width: 320,
        render: (_, record) => renderModelLimitUsage(record, t),
      },
    ],
    [t, historicalUsage],
  );

  return (
    <SideSheet
      title={
        <Space>
          <Tag color='cyan' shape='circle'>
            {t('用户用量')}
          </Tag>
          <Title heading={4} className='m-0'>
            {plan?.title || t('订阅套餐')}
          </Title>
        </Space>
      }
      visible={visible}
      onCancel={onCancel}
      closeIcon={null}
      width={isMobile ? '100%' : 1120}
      bodyStyle={{ padding: 16 }}
    >
      {renderCurrentSnapshot(currentSnapshot, records.length, t)}

      <div className='mb-3 flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
        <Space wrap>
          <Tag color='green' shape='circle'>
            {t('记录')} {records.length}
          </Tag>
          <Tag color='blue' shape='circle'>
            {t('当前显示')} {filteredRecords.length}
          </Tag>
        </Space>
        <Space wrap>
          <Select
            optionList={statusOptions}
            value={statusFilter}
            onChange={setStatusFilter}
            style={{ width: 130 }}
          />
          <Input
            value={keyword}
            onChange={setKeyword}
            showClear
            placeholder={t('搜索用户ID、用户名、邮箱')}
            style={{ width: isMobile ? '100%' : 240 }}
          />
          <Button loading={loading} onClick={loadUsage}>
            {t('刷新')}
          </Button>
        </Space>
      </div>

      <CardTable
        columns={columns}
        dataSource={paginatedRecords}
        loading={loading}
        rowKey={(row) => row?.subscription?.id}
        scroll={{ x: 'max-content' }}
        pagination={{
          currentPage,
          pageSize,
          total: filteredRecords.length,
          showSizeChanger: false,
          onPageChange: setCurrentPage,
        }}
        empty={
          <Empty
            image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
            darkModeImage={
              <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
            }
            description={t('暂无用户订阅')}
            style={{ padding: 30 }}
          />
        }
        size='middle'
      />
    </SideSheet>
  );
};

export default PlanUserUsageModal;
