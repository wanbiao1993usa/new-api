import React, { useState, useCallback, useMemo, useRef } from 'react';
import {
  Button,
  Input,
  InputNumber,
  Checkbox,
  Select,
  Typography,
  Popconfirm,
} from '@douyinfe/semi-ui';
import { IconPlus, IconDelete } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import CardTable from '../../../../components/common/ui/CardTable';

const { Text } = Typography;

let _idCounter = 0;
const uid = () => `gr_${++_idCounter}`;

function parseJSON(str, fallback) {
  if (!str || !str.trim()) return fallback;
  try {
    return JSON.parse(str);
  } catch {
    return fallback;
  }
}

const billingTypeOptions = [
  { value: 'default', label: '保持现状' },
  { value: 'subscription_only', label: '仅订阅' },
  { value: 'wallet_only', label: '仅余额' },
];

function parseBillingType(value) {
  if (value === 'subscription_only' || value === 'wallet_only') return value;
  return 'default';
}

function buildRows(
  groupRatioStr,
  userUsableGroupsStr,
  userVisibleGroupsStr,
  groupBillingTypeStr,
) {
  const ratioMap = parseJSON(groupRatioStr, {});
  const usableMap = parseJSON(userUsableGroupsStr, {});
  const visibleConfigured = !!(
    userVisibleGroupsStr && userVisibleGroupsStr.trim()
  );
  const visibleMap = visibleConfigured
    ? parseJSON(userVisibleGroupsStr, {})
    : {};
  const billingTypeMap = parseJSON(groupBillingTypeStr, {});

  const allNames = new Set([
    ...Object.keys(ratioMap),
    ...Object.keys(usableMap),
    ...Object.keys(visibleMap),
    ...Object.keys(billingTypeMap),
  ]);

  return Array.from(allNames).map((name) => ({
    _id: uid(),
    name,
    ratio: ratioMap[name] ?? 1,
    accessible: name in usableMap,
    visible: visibleConfigured ? name in visibleMap : name in usableMap,
    description: visibleMap[name] ?? usableMap[name] ?? '',
    billingType: parseBillingType(billingTypeMap[name]),
  }));
}

export function serializeGroupTable(rows) {
  const groupRatio = {};
  const userUsableGroups = {};
  const userVisibleGroups = {};
  const groupBillingType = {};

  rows.forEach((row) => {
    if (!row.name) return;
    groupRatio[row.name] = row.ratio;
    if (row.billingType && row.billingType !== 'default') {
      groupBillingType[row.name] = row.billingType;
    }
    if (row.accessible) {
      userUsableGroups[row.name] = row.description;
    }
    if (row.visible) {
      userVisibleGroups[row.name] = row.description;
    }
  });

  return {
    GroupRatio: JSON.stringify(groupRatio, null, 2),
    UserUsableGroups: JSON.stringify(userUsableGroups, null, 2),
    UserVisibleGroups: JSON.stringify(userVisibleGroups, null, 2),
    GroupBillingType: JSON.stringify(groupBillingType, null, 2),
  };
}

export default function GroupTable({
  groupRatio,
  userUsableGroups,
  userVisibleGroups,
  groupBillingType,
  onChange,
}) {
  const { t } = useTranslation();

  const [rows, setRows] = useState(() =>
    buildRows(
      groupRatio,
      userUsableGroups,
      userVisibleGroups,
      groupBillingType,
    ),
  );

  // Use functional setRows to keep updateRow/addRow/removeRow referentially
  // stable, preventing columns useMemo from rebuilding on every keystroke
  // which causes the Input cursor to jump to end (cursor reset bug).
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const emitAndSet = useCallback((updater) => {
    setRows((prev) => {
      const next = typeof updater === 'function' ? updater(prev) : updater;
      onChangeRef.current?.(serializeGroupTable(next));
      return next;
    });
  }, []);

  const updateRow = useCallback(
    (id, field, value) => {
      emitAndSet((prev) =>
        prev.map((r) => (r._id === id ? { ...r, [field]: value } : r)),
      );
    },
    [emitAndSet],
  );

  const addRow = useCallback(() => {
    emitAndSet((prev) => {
      const existingNames = new Set(prev.map((r) => r.name));
      let counter = 1;
      let newName = `group_${counter}`;
      while (existingNames.has(newName)) {
        counter++;
        newName = `group_${counter}`;
      }
      return [
        ...prev,
        {
          _id: uid(),
          name: newName,
          ratio: 1,
          accessible: true,
          visible: true,
          description: '',
          billingType: 'default',
        },
      ];
    });
  }, [emitAndSet]);

  const removeRow = useCallback(
    (id) => {
      emitAndSet((prev) => prev.filter((r) => r._id !== id));
    },
    [emitAndSet],
  );

  const groupNames = useMemo(() => rows.map((r) => r.name), [rows]);

  const duplicateNames = useMemo(() => {
    const counts = {};
    groupNames.forEach((n) => {
      counts[n] = (counts[n] || 0) + 1;
    });
    return new Set(Object.keys(counts).filter((k) => counts[k] > 1));
  }, [groupNames]);

  // Use ref so column render functions always read the latest duplicate set
  // without adding duplicateNames to columns deps (which would break cursor).
  const duplicateNamesRef = useRef(duplicateNames);
  duplicateNamesRef.current = duplicateNames;

  const columns = useMemo(
    () => [
      {
        title: t('分组名称'),
        dataIndex: 'name',
        key: 'name',
        width: 180,
        render: (_, record) => (
          <Input
            size='small'
            value={record.name}
            status={
              duplicateNamesRef.current.has(record.name) ? 'warning' : undefined
            }
            onChange={(v) => updateRow(record._id, 'name', v)}
          />
        ),
      },
      {
        title: t('倍率'),
        dataIndex: 'ratio',
        key: 'ratio',
        width: 120,
        render: (_, record) => (
          <InputNumber
            size='small'
            min={0}
            step={0.1}
            value={record.ratio}
            style={{ width: '100%' }}
            onChange={(v) => updateRow(record._id, 'ratio', v ?? 0)}
          />
        ),
      },
      {
        title: t('扣费类型'),
        dataIndex: 'billingType',
        key: 'billingType',
        width: 130,
        render: (_, record) => (
          <Select
            size='small'
            value={record.billingType || 'default'}
            optionList={billingTypeOptions.map((item) => ({
              ...item,
              label: t(item.label),
            }))}
            style={{ width: '100%' }}
            onChange={(v) => updateRow(record._id, 'billingType', v)}
          />
        ),
      },
      {
        title: t('可访问'),
        dataIndex: 'accessible',
        key: 'accessible',
        width: 90,
        align: 'center',
        render: (_, record) => (
          <Checkbox
            checked={record.accessible}
            onChange={(e) =>
              updateRow(record._id, 'accessible', e.target.checked)
            }
          />
        ),
      },
      {
        title: t('创建可见'),
        dataIndex: 'visible',
        key: 'visible',
        width: 100,
        align: 'center',
        render: (_, record) => (
          <Checkbox
            checked={record.visible}
            onChange={(e) => updateRow(record._id, 'visible', e.target.checked)}
          />
        ),
      },
      {
        title: t('描述'),
        dataIndex: 'description',
        key: 'description',
        render: (_, record) =>
          record.accessible || record.visible ? (
            <Input
              size='small'
              value={record.description}
              placeholder={t('分组描述')}
              onChange={(v) => updateRow(record._id, 'description', v)}
            />
          ) : (
            <Text type='tertiary' size='small'>
              -
            </Text>
          ),
      },
      {
        title: '',
        key: 'actions',
        width: 50,
        render: (_, record) => (
          <Popconfirm
            title={t('确认删除该分组？')}
            onConfirm={() => removeRow(record._id)}
            position='left'
          >
            <Button
              icon={<IconDelete />}
              type='danger'
              theme='borderless'
              size='small'
            />
          </Popconfirm>
        ),
      },
    ],
    [t, updateRow, removeRow],
  );

  return (
    <div>
      <CardTable
        columns={columns}
        dataSource={rows}
        rowKey='_id'
        hidePagination
        size='small'
        empty={<Text type='tertiary'>{t('暂无分组，点击下方按钮添加')}</Text>}
      />
      <div className='mt-3 flex justify-center'>
        <Button icon={<IconPlus />} theme='outline' onClick={addRow}>
          {t('添加分组')}
        </Button>
      </div>
      {duplicateNames.size > 0 && (
        <Text type='warning' size='small' className='mt-2 block'>
          {t('存在重复的分组名称：')}
          {Array.from(duplicateNames).join(', ')}
        </Text>
      )}
    </div>
  );
}
