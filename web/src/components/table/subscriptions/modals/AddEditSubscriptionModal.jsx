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

import React, { useEffect, useState, useRef } from 'react';
import {
  Avatar,
  Banner,
  Button,
  Card,
  Col,
  Form,
  InputNumber,
  Row,
  Select,
  SideSheet,
  Space,
  Spin,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconCalendarClock,
  IconAlertTriangle,
  IconClose,
  IconCreditCard,
  IconDelete,
  IconPlus,
  IconSave,
} from '@douyinfe/semi-icons';
import { Clock, RefreshCw } from 'lucide-react';
import { API, renderQuota, showError, showSuccess } from '../../../../helpers';
import {
  quotaToDisplayAmount,
  displayAmountToQuota,
} from '../../../../helpers/quota';
import { selectFilter } from '../../../../helpers/utils';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const durationUnitOptions = [
  { value: 'year', label: '年' },
  { value: 'month', label: '月' },
  { value: 'day', label: '日' },
  { value: 'hour', label: '小时' },
  { value: 'custom', label: '自定义(秒)' },
];

const resetPeriodOptions = [
  { value: 'never', label: '不重置' },
  { value: 'daily', label: '每天' },
  { value: 'weekly', label: '每周' },
  { value: 'monthly', label: '每月' },
  { value: 'custom', label: '自定义(秒)' },
];

function formatJSONSafe(value) {
  if (!value || !String(value).trim()) return '';
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

const MODEL_LIMIT_DEFAULT_KEY = '*';

function createModelLimitRow(model = '', amount = 0) {
  return {
    id: `${Date.now()}-${Math.random()}`,
    model,
    amount: Number(amount || 0),
  };
}

function parseModelLimitRows(value) {
  if (!value || !String(value).trim()) return [];
  try {
    const parsed = JSON.parse(value);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return null;
    }
    return Object.entries(parsed).map(([model, amount]) =>
      createModelLimitRow(model, Number(amount || 0)),
    );
  } catch {
    return null;
  }
}

function serializeModelLimitRows(rows) {
  const limits = {};
  rows.forEach((row) => {
    const model = String(row.model || '').trim();
    const amount = Number(row.amount || 0);
    if (!model || !Number.isFinite(amount) || amount < 0) return;
    limits[model] = Math.trunc(amount);
  });
  if (Object.keys(limits).length === 0) return '';
  return JSON.stringify(limits, null, 2);
}

const AddEditSubscriptionModal = ({
  visible,
  handleClose,
  editingPlan,
  placement = 'left',
  refresh,
  t,
}) => {
  const [loading, setLoading] = useState(false);
  const [groupOptions, setGroupOptions] = useState([]);
  const [groupLoading, setGroupLoading] = useState(false);
  const [modelOptions, setModelOptions] = useState([]);
  const [modelLoading, setModelLoading] = useState(false);
  const [modelLimitRows, setModelLimitRows] = useState([]);
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);
  const modelLimitSyncRef = useRef(false);
  const isEdit = editingPlan?.plan?.id !== undefined;
  const formKey = isEdit ? `edit-${editingPlan?.plan?.id}` : 'create';

  const getInitValues = () => ({
    title: '',
    subtitle: '',
    price_amount: 0,
    currency: 'USD',
    duration_unit: 'month',
    duration_value: 1,
    custom_seconds: 0,
    quota_reset_period: 'never',
    quota_reset_custom_seconds: 0,
    enabled: true,
    sort_order: 0,
    max_purchase_per_user: 0,
    total_amount: 0,
    model_amount_limits: '',
    upgrade_group: '',
    stripe_price_id: '',
    creem_product_id: '',
  });

  const buildFormValues = () => {
    const base = getInitValues();
    if (editingPlan?.plan?.id === undefined) return base;
    const p = editingPlan.plan || {};
    return {
      ...base,
      title: p.title || '',
      subtitle: p.subtitle || '',
      price_amount: Number(p.price_amount || 0),
      currency: 'USD',
      duration_unit: p.duration_unit || 'month',
      duration_value: Number(p.duration_value || 1),
      custom_seconds: Number(p.custom_seconds || 0),
      quota_reset_period: p.quota_reset_period || 'never',
      quota_reset_custom_seconds: Number(p.quota_reset_custom_seconds || 0),
      enabled: p.enabled !== false,
      sort_order: Number(p.sort_order || 0),
      max_purchase_per_user: Number(p.max_purchase_per_user || 0),
      total_amount: Number(
        quotaToDisplayAmount(p.total_amount || 0).toFixed(2),
      ),
      model_amount_limits: formatJSONSafe(p.model_amount_limits),
      upgrade_group: p.upgrade_group || '',
      stripe_price_id: p.stripe_price_id || '',
      creem_product_id: p.creem_product_id || '',
    };
  };

  useEffect(() => {
    if (!visible) return;
    setGroupLoading(true);
    API.get('/api/group')
      .then((res) => {
        if (res.data?.success) {
          setGroupOptions(res.data?.data || []);
        } else {
          setGroupOptions([]);
        }
      })
      .catch(() => setGroupOptions([]))
      .finally(() => setGroupLoading(false));
  }, [visible]);

  useEffect(() => {
    if (!visible) return;
    const raw = editingPlan?.plan?.model_amount_limits || '';
    setModelLimitRows(parseModelLimitRows(formatJSONSafe(raw)) || []);
  }, [visible, editingPlan?.plan?.id, editingPlan?.plan?.model_amount_limits]);

  useEffect(() => {
    if (!visible) return;
    setModelLoading(true);
    API.get('/api/models/?page_size=1000')
      .then((res) => {
        if (!res.data?.success) {
          setModelOptions([]);
          return;
        }
        const items = res.data?.data?.items || res.data?.data || [];
        const options = (Array.isArray(items) ? items : [])
          .map((item) => String(item?.model_name || '').trim())
          .filter(Boolean)
          .filter((model, index, list) => list.indexOf(model) === index)
          .map((model) => ({ label: model, value: model }));
        setModelOptions(options);
      })
      .catch(() => {
        setModelOptions([]);
        showError(t('加载模型列表失败'));
      })
      .finally(() => setModelLoading(false));
  }, [visible, t]);

  const getModelLimitOptions = (currentModel) => {
    const options = [
      { label: t('默认模型 (*)'), value: MODEL_LIMIT_DEFAULT_KEY },
      ...modelOptions,
    ];
    const model = String(currentModel || '').trim();
    if (model && !options.some((item) => item.value === model)) {
      options.push({ label: model, value: model });
    }
    return options;
  };

  const syncModelLimitRows = (rows) => {
    setModelLimitRows(rows);
    modelLimitSyncRef.current = true;
    formApiRef.current?.setValue(
      'model_amount_limits',
      serializeModelLimitRows(rows),
    );
    setTimeout(() => {
      modelLimitSyncRef.current = false;
    }, 0);
  };

  const addModelLimitRow = () => {
    syncModelLimitRows([...modelLimitRows, createModelLimitRow('', 0)]);
  };

  const updateModelLimitRow = (rowId, patch) => {
    const nextRows = modelLimitRows.map((row) =>
      row.id === rowId ? { ...row, ...patch } : row,
    );
    syncModelLimitRows(nextRows);
  };

  const removeModelLimitRow = (rowId) => {
    syncModelLimitRows(modelLimitRows.filter((row) => row.id !== rowId));
  };

  const handleModelLimitJsonChange = (value) => {
    if (modelLimitSyncRef.current) return;
    const parsedRows = parseModelLimitRows(value);
    if (parsedRows) {
      setModelLimitRows(parsedRows);
    } else if (!value || String(value).trim() === '') {
      setModelLimitRows([]);
    }
  };

  const submit = async (values) => {
    if (!values.title || values.title.trim() === '') {
      showError(t('套餐标题不能为空'));
      return;
    }
    setLoading(true);
    try {
      const payload = {
        plan: {
          ...values,
          price_amount: Number(values.price_amount || 0),
          currency: 'USD',
          duration_value: Number(values.duration_value || 0),
          custom_seconds: Number(values.custom_seconds || 0),
          quota_reset_period: values.quota_reset_period || 'never',
          quota_reset_custom_seconds:
            values.quota_reset_period === 'custom'
              ? Number(values.quota_reset_custom_seconds || 0)
              : 0,
          sort_order: Number(values.sort_order || 0),
          max_purchase_per_user: Number(values.max_purchase_per_user || 0),
          total_amount: displayAmountToQuota(values.total_amount),
          model_amount_limits: values.model_amount_limits || '',
          upgrade_group: values.upgrade_group || '',
        },
      };
      if (editingPlan?.plan?.id) {
        const res = await API.put(
          `/api/subscription/admin/plans/${editingPlan.plan.id}`,
          payload,
        );
        if (res.data?.success) {
          showSuccess(t('更新成功'));
          handleClose();
          refresh?.();
        } else {
          showError(res.data?.message || t('更新失败'));
        }
      } else {
        const res = await API.post('/api/subscription/admin/plans', payload);
        if (res.data?.success) {
          showSuccess(t('创建成功'));
          handleClose();
          refresh?.();
        } else {
          showError(res.data?.message || t('创建失败'));
        }
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <SideSheet
        placement={placement}
        title={
          <Space>
            {isEdit ? (
              <Tag color='blue' shape='circle'>
                {t('更新')}
              </Tag>
            ) : (
              <Tag color='green' shape='circle'>
                {t('新建')}
              </Tag>
            )}
            <Title heading={4} className='m-0'>
              {isEdit ? t('更新套餐信息') : t('创建新的订阅套餐')}
            </Title>
          </Space>
        }
        bodyStyle={{ padding: '0' }}
        visible={visible}
        width={isMobile ? '100%' : 600}
        footer={
          <div className='flex justify-end bg-white'>
            <Space>
              <Button
                theme='solid'
                onClick={() => formApiRef.current?.submitForm()}
                icon={<IconSave />}
                loading={loading}
              >
                {t('提交')}
              </Button>
              <Button
                theme='light'
                type='primary'
                onClick={handleClose}
                icon={<IconClose />}
              >
                {t('取消')}
              </Button>
            </Space>
          </div>
        }
        closeIcon={null}
        onCancel={handleClose}
      >
        <Spin spinning={loading}>
          <Form
            key={formKey}
            initValues={buildFormValues()}
            getFormApi={(api) => (formApiRef.current = api)}
            onSubmit={submit}
          >
            {({ values }) => (
              <div className='p-2'>
                {/* 基本信息 */}
                <Card className='!rounded-2xl shadow-sm border-0 mb-4'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='blue'
                      className='mr-2 shadow-md'
                    >
                      <IconCalendarClock size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('基本信息')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('套餐的基本信息和定价')}
                      </div>
                    </div>
                  </div>

                  <Row gutter={12}>
                    <Col span={24}>
                      <Form.Input
                        field='title'
                        label={t('套餐标题')}
                        placeholder={t('例如：基础套餐')}
                        required
                        rules={[
                          { required: true, message: t('请输入套餐标题') },
                        ]}
                        showClear
                      />
                    </Col>

                    <Col span={24}>
                      <Form.Input
                        field='subtitle'
                        label={t('套餐副标题')}
                        placeholder={t('例如：适合轻度使用')}
                        showClear
                      />
                    </Col>

                    <Col span={12}>
                      <Form.InputNumber
                        field='price_amount'
                        label={t('实付金额')}
                        required
                        min={0}
                        precision={2}
                        rules={[{ required: true, message: t('请输入金额') }]}
                        style={{ width: '100%' }}
                      />
                    </Col>

                    <Col span={12}>
                      <Form.InputNumber
                        field='total_amount'
                        label={t('总额度')}
                        required
                        min={0}
                        precision={2}
                        rules={[{ required: true, message: t('请输入总额度') }]}
                        extraText={`${t('0 表示不限')} · ${t('原生额度')}：${displayAmountToQuota(
                          values.total_amount,
                        )}`}
                        style={{ width: '100%' }}
                      />
                    </Col>

                    <Col span={12}>
                      <Form.Select
                        field='upgrade_group'
                        label={t('升级分组')}
                        showClear
                        loading={groupLoading}
                        placeholder={t('不升级')}
                        extraText={t(
                          '购买或手动新增订阅会升级到该分组；当套餐失效/过期或手动作废/删除后，将回退到升级前分组。回退不会立即生效，通常会有几分钟延迟。',
                        )}
                      >
                        <Select.Option value=''>{t('不升级')}</Select.Option>
                        {(groupOptions || []).map((g) => (
                          <Select.Option key={g} value={g}>
                            {g}
                          </Select.Option>
                        ))}
                      </Form.Select>
                    </Col>

                    <Col span={12}>
                      <Form.Input
                        field='currency'
                        label={t('币种')}
                        disabled
                        extraText={t('由全站货币展示设置统一控制')}
                      />
                    </Col>

                    <Col span={12}>
                      <Form.InputNumber
                        field='sort_order'
                        label={t('排序')}
                        precision={0}
                        style={{ width: '100%' }}
                      />
                    </Col>

                    <Col span={12}>
                      <Form.InputNumber
                        field='max_purchase_per_user'
                        label={t('购买上限')}
                        min={0}
                        precision={0}
                        extraText={t('0 表示不限')}
                        style={{ width: '100%' }}
                      />
                    </Col>

                    <Col span={12}>
                      <Form.Switch
                        field='enabled'
                        label={t('启用状态')}
                        size='large'
                      />
                    </Col>
                  </Row>
                </Card>

                {/* 有效期设置 */}
                <Card className='!rounded-2xl shadow-sm border-0 mb-4'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='green'
                      className='mr-2 shadow-md'
                    >
                      <Clock size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('有效期设置')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('配置套餐的有效时长')}
                      </div>
                    </div>
                  </div>

                  <Row gutter={12}>
                    <Col span={12}>
                      <Form.Select
                        field='duration_unit'
                        label={t('有效期单位')}
                        required
                        rules={[{ required: true }]}
                      >
                        {durationUnitOptions.map((o) => (
                          <Select.Option key={o.value} value={o.value}>
                            {o.label}
                          </Select.Option>
                        ))}
                      </Form.Select>
                    </Col>

                    <Col span={12}>
                      {values.duration_unit === 'custom' ? (
                        <Form.InputNumber
                          field='custom_seconds'
                          label={t('自定义秒数')}
                          required
                          min={1}
                          precision={0}
                          rules={[{ required: true, message: t('请输入秒数') }]}
                          style={{ width: '100%' }}
                        />
                      ) : (
                        <Form.InputNumber
                          field='duration_value'
                          label={t('有效期数值')}
                          required
                          min={1}
                          precision={0}
                          rules={[{ required: true, message: t('请输入数值') }]}
                          style={{ width: '100%' }}
                        />
                      )}
                    </Col>
                  </Row>
                </Card>

                {/* 额度重置 */}
                <Card className='!rounded-2xl shadow-sm border-0 mb-4'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='orange'
                      className='mr-2 shadow-md'
                    >
                      <RefreshCw size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('额度重置')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('支持周期性重置套餐权益额度')}
                      </div>
                    </div>
                  </div>

                  <Row gutter={12}>
                    <Col span={12}>
                      <Form.Select
                        field='quota_reset_period'
                        label={t('重置周期')}
                      >
                        {resetPeriodOptions.map((o) => (
                          <Select.Option key={o.value} value={o.value}>
                            {o.label}
                          </Select.Option>
                        ))}
                      </Form.Select>
                    </Col>
                    <Col span={12}>
                      {values.quota_reset_period === 'custom' ? (
                        <Form.InputNumber
                          field='quota_reset_custom_seconds'
                          label={t('自定义秒数')}
                          required
                          min={60}
                          precision={0}
                          rules={[{ required: true, message: t('请输入秒数') }]}
                          style={{ width: '100%' }}
                        />
                      ) : (
                        <Form.InputNumber
                          field='quota_reset_custom_seconds'
                          label={t('自定义秒数')}
                          min={0}
                          precision={0}
                          style={{ width: '100%' }}
                          disabled
                        />
                      )}
                    </Col>
                  </Row>
                </Card>

                {/* 模型限额 */}
                <Card className='!rounded-2xl shadow-sm border-0 mb-4'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='cyan'
                      className='mr-2 shadow-md'
                    >
                      <IconCreditCard size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('模型限额')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('按模型限制订阅周期内可消耗额度')}
                      </div>
                    </div>
                  </div>

                  <Banner
                    type='info'
                    closeIcon={null}
                    icon={
                      <IconAlertTriangle
                        size='large'
                        style={{ color: 'var(--semi-color-info)' }}
                      />
                    }
                    className='!rounded-xl mb-3'
                    description={t(
                      '模型限额按运行时套餐配置生效，修改后会影响该套餐下仍在生效的订阅；用户不可用的模型不会在购买页展示。',
                    )}
                  />

                  <Row gutter={12}>
                    <Col span={24}>
                      <div className='mb-3'>
                        <div className='flex items-center justify-between mb-2'>
                          <Text strong>{t('限额规则')}</Text>
                          <Button
                            size='small'
                            type='primary'
                            theme='light'
                            icon={<IconPlus />}
                            onClick={addModelLimitRow}
                          >
                            {t('添加模型')}
                          </Button>
                        </div>

                        {modelLimitRows.length === 0 ? (
                          <div className='rounded-lg border border-dashed border-gray-200 px-3 py-3 text-sm text-gray-500'>
                            {t('未配置模型限额')}
                          </div>
                        ) : (
                          <div className='flex flex-col gap-2'>
                            {modelLimitRows.map((row) => (
                              <div
                                key={row.id}
                                className='grid grid-cols-1 sm:grid-cols-[1fr_160px_36px] gap-2 sm:items-center'
                              >
                                <Select
                                  size='small'
                                  value={row.model || undefined}
                                  optionList={getModelLimitOptions(row.model)}
                                  filter={selectFilter}
                                  allowCreate
                                  loading={modelLoading}
                                  placeholder={t('选择或输入模型')}
                                  style={{ width: '100%' }}
                                  onChange={(value) =>
                                    updateModelLimitRow(row.id, {
                                      model: value || '',
                                    })
                                  }
                                />
                                <div>
                                  <Tooltip
                                    content={`${t('展示额度')}：${renderQuota(
                                      Number(row.amount || 0),
                                    )}`}
                                  >
                                    <InputNumber
                                      size='small'
                                      value={row.amount}
                                      min={0}
                                      precision={0}
                                      style={{ width: '100%' }}
                                      onChange={(value) =>
                                        updateModelLimitRow(row.id, {
                                          amount: value ?? 0,
                                        })
                                      }
                                    />
                                  </Tooltip>
                                  <Text type='tertiary' size='small'>
                                    {t('展示')}：
                                    {renderQuota(Number(row.amount || 0))}
                                  </Text>
                                </div>
                                <Button
                                  size='small'
                                  type='danger'
                                  theme='borderless'
                                  icon={<IconDelete />}
                                  aria-label={t('删除模型限额')}
                                  onClick={() => removeModelLimitRow(row.id)}
                                />
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    </Col>

                    <Col span={24}>
                      <Form.TextArea
                        field='model_amount_limits'
                        label={t('模型限额 JSON')}
                        placeholder={t('{"gpt-5.5": 6250000, "*": 1000000}')}
                        autosize={{ minRows: 4, maxRows: 10 }}
                        onChange={handleModelLimitJsonChange}
                        extraText={t(
                          '键为平台模型名，值为原生额度；* 表示默认模型限额。留空表示不启用模型限额。',
                        )}
                        rules={[
                          {
                            validator: (rule, value) => {
                              if (!value || value.trim() === '') return true;
                              try {
                                const parsed = JSON.parse(value);
                                return (
                                  parsed &&
                                  typeof parsed === 'object' &&
                                  !Array.isArray(parsed) &&
                                  Object.values(parsed).every(
                                    (v) =>
                                      typeof v === 'number' &&
                                      Number.isInteger(v) &&
                                      v >= 0,
                                  )
                                );
                              } catch {
                                return false;
                              }
                            },
                            message: t('请输入合法的模型限额 JSON'),
                          },
                        ]}
                      />
                    </Col>
                  </Row>
                </Card>

                {/* 第三方支付配置 */}
                <Card className='!rounded-2xl shadow-sm border-0 mb-4'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='purple'
                      className='mr-2 shadow-md'
                    >
                      <IconCreditCard size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('第三方支付配置')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('Stripe/Creem 商品ID（可选）')}
                      </div>
                    </div>
                  </div>

                  <Row gutter={12}>
                    <Col span={24}>
                      <Form.Input
                        field='stripe_price_id'
                        label='Stripe PriceId'
                        placeholder='price_...'
                        showClear
                      />
                    </Col>

                    <Col span={24}>
                      <Form.Input
                        field='creem_product_id'
                        label='Creem ProductId'
                        placeholder='prod_...'
                        showClear
                      />
                    </Col>
                  </Row>
                </Card>
              </div>
            )}
          </Form>
        </Spin>
      </SideSheet>
    </>
  );
};

export default AddEditSubscriptionModal;
