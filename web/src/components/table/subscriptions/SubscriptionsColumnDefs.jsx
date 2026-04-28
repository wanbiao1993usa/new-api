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

import React from 'react';
import {
  Button,
  Modal,
  Space,
  Tag,
  Typography,
  Popover,
  Divider,
  Badge,
  Tooltip,
} from '@douyinfe/semi-ui';
import { renderQuota } from '../../../helpers';
import { convertUSDToCurrency } from '../../../helpers/render';

const { Text } = Typography;

function formatDuration(plan, t) {
  if (!plan) return '';
  const u = plan.duration_unit || 'month';
  if (u === 'custom') {
    return `${t('自定义')} ${plan.custom_seconds || 0}s`;
  }
  const unitMap = {
    year: t('年'),
    month: t('月'),
    day: t('日'),
    hour: t('小时'),
  };
  return `${plan.duration_value || 0}${unitMap[u] || u}`;
}

function formatResetPeriod(plan, t) {
  const period = plan?.quota_reset_period || 'never';
  if (period === 'daily') return t('每天');
  if (period === 'weekly') return t('每周');
  if (period === 'monthly') return t('每月');
  if (period === 'custom') {
    const seconds = Number(plan?.quota_reset_custom_seconds || 0);
    if (seconds >= 86400) return `${Math.floor(seconds / 86400)} ${t('天')}`;
    if (seconds >= 3600) return `${Math.floor(seconds / 3600)} ${t('小时')}`;
    if (seconds >= 60) return `${Math.floor(seconds / 60)} ${t('分钟')}`;
    return `${seconds} ${t('秒')}`;
  }
  return t('不重置');
}

function parseModelAmountLimits(plan) {
  const raw = plan?.model_amount_limits;
  if (!raw || !String(raw).trim()) return {};
  try {
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? parsed
      : {};
  } catch {
    return {};
  }
}

function renderModelAmountLimits(text, record, t) {
  const limits = parseModelAmountLimits(record?.plan);
  const entries = Object.entries(limits);
  if (!entries.length) {
    return <Text type='tertiary'>{t('无')}</Text>;
  }
  const content = (
    <div style={{ minWidth: 240, maxWidth: 360 }}>
      <Text strong>{t('模型限额')}</Text>
      <div style={{ marginTop: 8, display: 'grid', gap: 6 }}>
        {entries.map(([modelName, amount]) => (
          <div
            key={modelName}
            style={{
              display: 'grid',
              gridTemplateColumns: '1fr auto',
              gap: 12,
            }}
          >
            <Text ellipsis={{ showTooltip: true }}>
              {modelName === '*' ? t('其他模型') : modelName}
            </Text>
            <Tooltip content={`${t('原生额度')}：${amount}`}>
              <Text>{renderQuota(Number(amount || 0))}</Text>
            </Tooltip>
          </div>
        ))}
      </div>
    </div>
  );
  return (
    <Popover content={content} position='leftTop' showArrow>
      <Tag color='cyan' shape='circle'>
        {t('已配置')} {entries.length}
      </Tag>
    </Popover>
  );
}

const renderPlanTitle = (text, record, t) => {
  const subtitle = record?.plan?.subtitle;
  const plan = record?.plan;
  const popoverContent = (
    <div style={{ width: 260 }}>
      <Text strong>{text}</Text>
      {subtitle && (
        <Text type='tertiary' style={{ display: 'block', marginTop: 4 }}>
          {subtitle}
        </Text>
      )}
      <Divider margin={12} />
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
        <Text type='tertiary'>{t('价格')}</Text>
        <Text strong style={{ color: 'var(--semi-color-success)' }}>
          {convertUSDToCurrency(Number(plan?.price_amount || 0), 2)}
        </Text>
        <Text type='tertiary'>{t('总额度')}</Text>
        {plan?.total_amount > 0 ? (
          <Tooltip content={`${t('原生额度')}：${plan.total_amount}`}>
            <Text>{renderQuota(plan.total_amount)}</Text>
          </Tooltip>
        ) : (
          <Text>{t('不限')}</Text>
        )}
        <Text type='tertiary'>{t('升级分组')}</Text>
        <Text>{plan?.upgrade_group ? plan.upgrade_group : t('不升级')}</Text>
        <Text type='tertiary'>{t('购买上限')}</Text>
        <Text>
          {plan?.max_purchase_per_user > 0
            ? plan.max_purchase_per_user
            : t('不限')}
        </Text>
        <Text type='tertiary'>{t('有效期')}</Text>
        <Text>{formatDuration(plan, t)}</Text>
        <Text type='tertiary'>{t('重置')}</Text>
        <Text>{formatResetPeriod(plan, t)}</Text>
      </div>
    </div>
  );

  return (
    <Popover content={popoverContent} position='rightTop' showArrow>
      <div style={{ cursor: 'pointer', maxWidth: 180 }}>
        <Text strong ellipsis={{ showTooltip: false }}>
          {text}
        </Text>
        {subtitle && (
          <Text
            type='tertiary'
            ellipsis={{ showTooltip: false }}
            style={{ display: 'block' }}
          >
            {subtitle}
          </Text>
        )}
      </div>
    </Popover>
  );
};

const renderPrice = (text) => {
  return (
    <Text strong style={{ color: 'var(--semi-color-success)' }}>
      {convertUSDToCurrency(Number(text || 0), 2)}
    </Text>
  );
};

const renderPurchaseLimit = (text, record, t) => {
  const limit = Number(record?.plan?.max_purchase_per_user || 0);
  return (
    <Text type={limit > 0 ? 'secondary' : 'tertiary'}>
      {limit > 0 ? limit : t('不限')}
    </Text>
  );
};

const renderDuration = (text, record, t) => {
  return <Text type='secondary'>{formatDuration(record?.plan, t)}</Text>;
};

const renderEnabled = (text, record, t) => {
  return text ? (
    <Tag
      color='white'
      shape='circle'
      type='light'
      prefixIcon={<Badge dot type='success' />}
    >
      {t('启用')}
    </Tag>
  ) : (
    <Tag
      color='white'
      shape='circle'
      type='light'
      prefixIcon={<Badge dot type='danger' />}
    >
      {t('禁用')}
    </Tag>
  );
};

const renderTotalAmount = (text, record, t) => {
  const total = Number(record?.plan?.total_amount || 0);
  return (
    <Text type={total > 0 ? 'secondary' : 'tertiary'}>
      {total > 0 ? (
        <Tooltip content={`${t('原生额度')}：${total}`}>
          <span>{renderQuota(total)}</span>
        </Tooltip>
      ) : (
        t('不限')
      )}
    </Text>
  );
};

const renderUpgradeGroup = (text, record, t) => {
  const group = record?.plan?.upgrade_group || '';
  return (
    <Text type={group ? 'secondary' : 'tertiary'}>
      {group ? group : t('不升级')}
    </Text>
  );
};

const renderResetPeriod = (text, record, t) => {
  const period = record?.plan?.quota_reset_period || 'never';
  const isNever = period === 'never';
  return (
    <Text type={isNever ? 'tertiary' : 'secondary'}>
      {formatResetPeriod(record?.plan, t)}
    </Text>
  );
};

const renderPaymentConfig = (text, record, t, enableEpay) => {
  const hasStripe = !!record?.plan?.stripe_price_id;
  const hasCreem = !!record?.plan?.creem_product_id;
  const hasEpay = !!enableEpay;

  return (
    <Space spacing={4}>
      {hasStripe && (
        <Tag color='violet' shape='circle'>
          Stripe
        </Tag>
      )}
      {hasCreem && (
        <Tag color='cyan' shape='circle'>
          Creem
        </Tag>
      )}
      {hasEpay && (
        <Tag color='light-green' shape='circle'>
          {t('易支付')}
        </Tag>
      )}
    </Space>
  );
};

const renderOperations = (text, record, { openEdit, setPlanEnabled, t }) => {
  const isEnabled = record?.plan?.enabled;

  const handleToggle = () => {
    if (isEnabled) {
      Modal.confirm({
        title: t('确认禁用'),
        content: t('禁用后用户端不再展示，但历史订单不受影响。是否继续？'),
        centered: true,
        onOk: () => setPlanEnabled(record, false),
      });
    } else {
      Modal.confirm({
        title: t('确认启用'),
        content: t('启用后套餐将在用户端展示。是否继续？'),
        centered: true,
        onOk: () => setPlanEnabled(record, true),
      });
    }
  };

  return (
    <Space spacing={8}>
      <Button
        theme='light'
        type='tertiary'
        size='small'
        onClick={() => openEdit(record)}
      >
        {t('编辑')}
      </Button>
      {isEnabled ? (
        <Button theme='light' type='danger' size='small' onClick={handleToggle}>
          {t('禁用')}
        </Button>
      ) : (
        <Button
          theme='light'
          type='primary'
          size='small'
          onClick={handleToggle}
        >
          {t('启用')}
        </Button>
      )}
    </Space>
  );
};

export const getSubscriptionsColumns = ({
  t,
  openEdit,
  setPlanEnabled,
  enableEpay,
}) => {
  return [
    {
      title: 'ID',
      key: 'id',
      width: 60,
      render: (_, record) => <Text type='tertiary'>#{record?.plan?.id}</Text>,
    },
    {
      title: t('套餐'),
      key: 'title',
      width: 200,
      render: (_, record) => renderPlanTitle(record?.plan?.title, record, t),
    },
    {
      title: t('价格'),
      key: 'price',
      width: 100,
      render: (_, record) => renderPrice(record?.plan?.price_amount),
    },
    {
      title: t('购买上限'),
      width: 90,
      render: (text, record) => renderPurchaseLimit(text, record, t),
    },
    {
      title: t('优先级'),
      key: 'sort_order',
      width: 80,
      render: (_, record) => (
        <Text type='tertiary'>{Number(record?.plan?.sort_order || 0)}</Text>
      ),
    },
    {
      title: t('有效期'),
      width: 100,
      render: (text, record) => renderDuration(text, record, t),
    },
    {
      title: t('重置'),
      width: 80,
      render: (text, record) => renderResetPeriod(text, record, t),
    },
    {
      title: t('状态'),
      key: 'enabled',
      width: 80,
      render: (_, record) => renderEnabled(record?.plan?.enabled, record, t),
    },
    {
      title: t('支付渠道'),
      width: 180,
      render: (text, record) =>
        renderPaymentConfig(text, record, t, enableEpay),
    },
    {
      title: t('总额度'),
      width: 100,
      render: (text, record) => renderTotalAmount(text, record, t),
    },
    {
      title: t('模型限额'),
      width: 110,
      render: (text, record) => renderModelAmountLimits(text, record, t),
    },
    {
      title: t('升级分组'),
      width: 100,
      render: (text, record) => renderUpgradeGroup(text, record, t),
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      fixed: 'right',
      width: 160,
      render: (text, record) =>
        renderOperations(text, record, { openEdit, setPlanEnabled, t }),
    },
  ];
};
