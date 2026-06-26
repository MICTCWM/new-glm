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

import React, { useState, useEffect, useMemo, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Modal,
  Button,
  Table,
  Tag,
  Typography,
  Space,
  Popconfirm,
  Spin,
  Form,
  Select,
  InputNumber,
  Input,
  Empty,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
} from '../../../../helpers';
import { buildRuleConfig, parseRuleConfig } from './BatchResetRuleModal';

const { Text } = Typography;

const RULE_TYPES = [
  'daily',
  'weekly',
  'monthly',
  'custom_interval',
  'specific_time',
];

const WEEKDAYS = [0, 1, 2, 3, 4, 5, 6];

const ruleTypeLabel = (type, t) => {
  switch (type) {
    case 'daily':
      return t('每天');
    case 'weekly':
      return t('每周');
    case 'monthly':
      return t('每月');
    case 'custom_interval':
      return t('自定义间隔');
    case 'specific_time':
      return t('定点时间');
    default:
      return type;
  }
};

const weekdayLabel = (d, t) => {
  const map = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
  return t(map[d] ?? '周日');
};

// 根据 rule_type 与 config 生成可读摘要
const renderRuleConfigSummary = (ruleType, ruleConfig, t) => {
  if (!ruleConfig) return '-';
  let config = ruleConfig;
  if (typeof ruleConfig === 'string') {
    try {
      config = JSON.parse(ruleConfig);
    } catch (e) {
      return '-';
    }
  }
  switch (ruleType) {
    case 'daily':
      return `${t('每天')} ${String(config.hour ?? 0).padStart(2, '0')}:${String(config.minute ?? 0).padStart(2, '0')}`;
    case 'weekly':
      return `${weekdayLabel(config.weekday ?? 0, t)} ${String(config.hour ?? 0).padStart(2, '0')}:${String(config.minute ?? 0).padStart(2, '0')}`;
    case 'monthly':
      return `${t('每月')} ${config.day_of_month ?? 1} ${t('号')} ${String(config.hour ?? 0).padStart(2, '0')}:${String(config.minute ?? 0).padStart(2, '0')}`;
    case 'custom_interval':
      return `${t('每隔')} ${config.interval_seconds ?? 3600} ${t('秒')}`;
    case 'specific_time':
      return timestamp2string(config.specific_time ?? 0);
    default:
      return '-';
  }
};

const renderResetValue = (value, t) => {
  if (value === undefined || value === null || value <= 0) {
    return <Tag color='grey'>{t('保持不变')}</Tag>;
  }
  return <Tag color='blue'>{value}</Tag>;
};

const ChannelResetRuleModal = ({ visible, channelId, onClose, t: tProp }) => {
  const { t: tHook } = useTranslation();
  const t = tProp || tHook;

  const [loading, setLoading] = useState(false);
  const [rules, setRules] = useState([]);
  const [submitting, setSubmitting] = useState(false);

  // 编辑/新增表单状态
  const [formVisible, setFormVisible] = useState(false);
  const [editingRule, setEditingRule] = useState(null);
  const [ruleType, setRuleType] = useState('daily');
  const [formApi, setFormApi] = useState(null);

  const loadRules = useCallback(async () => {
    if (!channelId) return;
    setLoading(true);
    try {
      const res = await API.get(`/api/channel/${channelId}/reset_rules`);
      if (res.data.success) {
        const data = res.data.data;
        setRules(Array.isArray(data) ? data : data?.rules || []);
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error?.message || t('获取重置规则失败'));
    } finally {
      setLoading(false);
    }
  }, [channelId, t]);

  useEffect(() => {
    if (visible && channelId) {
      loadRules();
    }
    if (!visible) {
      setRules([]);
      setFormVisible(false);
      setEditingRule(null);
    }
  }, [visible, channelId, loadRules]);

  const openAddForm = () => {
    setEditingRule(null);
    setRuleType('daily');
    setFormVisible(true);
  };

  const openEditForm = (rule) => {
    setEditingRule(rule);
    setRuleType(rule.rule_type || 'daily');
    setFormVisible(true);
  };

  const closeForm = () => {
    setFormVisible(false);
    setEditingRule(null);
  };

  const getFormInitValues = () => {
    if (editingRule) {
      const parsed = parseRuleConfig(editingRule.rule_type, editingRule.rule_config);
      return {
        rule_type: editingRule.rule_type,
        hour: parsed.hour ?? 3,
        minute: parsed.minute ?? 0,
        weekday: parsed.weekday ?? 1,
        day_of_month: parsed.day_of_month ?? 1,
        interval_seconds: parsed.interval_seconds ?? 3600,
        specific_time: parsed.specific_time ?? null,
        reset_value: editingRule.reset_value ?? 0,
        remark: editingRule.remark || '',
      };
    }
    return {
      rule_type: 'daily',
      hour: 3,
      minute: 0,
      weekday: 1,
      day_of_month: 1,
      interval_seconds: 3600,
      specific_time: null,
      reset_value: 0,
      remark: '',
    };
  };

  const handleSubmit = async () => {
    if (!channelId) return;
    const values = formApi ? formApi.getValues() : {};
    const ruleConfig = buildRuleConfig(ruleType, values);
    const resetValue =
      values.reset_value === undefined || values.reset_value === null
        ? 0
        : values.reset_value;
    const remark = values.remark || '';
    const payload = {
      channel_id: channelId,
      rule_type: ruleType,
      rule_config: ruleConfig,
      reset_value: resetValue,
      enabled: editingRule ? editingRule.enabled : true,
      remark,
    };
    setSubmitting(true);
    try {
      let res;
      if (editingRule) {
        res = await API.put('/api/channel/reset_rule', {
          id: editingRule.id,
          ...payload,
        });
      } else {
        res = await API.post('/api/channel/reset_rule', payload);
      }
      if (res.data.success) {
        showSuccess(editingRule ? t('更新成功') : t('创建成功'));
        closeForm();
        await loadRules();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error?.message || t('操作失败'));
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (ruleId) => {
    try {
      const res = await API.delete(`/api/channel/reset_rule/${ruleId}`);
      if (res.data.success) {
        showSuccess(t('删除成功'));
        await loadRules();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error?.message || t('删除失败'));
    }
  };

  const handleToggleEnabled = async (rule) => {
    try {
      const res = await API.put('/api/channel/reset_rule', {
        id: rule.id,
        channel_id: channelId,
        rule_type: rule.rule_type,
        rule_config: rule.rule_config,
        reset_value: rule.reset_value,
        enabled: !rule.enabled,
        remark: rule.remark || '',
      });
      if (res.data.success) {
        showSuccess(t('更新成功'));
        await loadRules();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error?.message || t('操作失败'));
    }
  };

  const ruleTypeOptions = useMemo(
    () =>
      RULE_TYPES.map((type) => ({
        value: type,
        label: ruleTypeLabel(type, t),
      })),
    [t],
  );

  const weekdayOptions = useMemo(
    () =>
      WEEKDAYS.map((d) => ({
        value: d,
        label: weekdayLabel(d, t),
      })),
    [t],
  );

  const columns = [
    {
      title: t('规则类型'),
      dataIndex: 'rule_type',
      width: 110,
      render: (text) => (
        <Tag color='light-blue' shape='circle'>
          {ruleTypeLabel(text, t)}
        </Tag>
      ),
    },
    {
      title: t('配置'),
      dataIndex: 'rule_config',
      render: (text, record) =>
        renderRuleConfigSummary(record.rule_type, text, t),
    },
    {
      title: t('下次重置时间'),
      dataIndex: 'next_reset_time',
      width: 180,
      render: (text) => {
        if (!text || text <= 0) return '-';
        return timestamp2string(text);
      },
    },
    {
      title: t('重置后配额值'),
      dataIndex: 'reset_value',
      width: 130,
      render: (text) => renderResetValue(text, t),
    },
    {
      title: t('状态'),
      dataIndex: 'enabled',
      width: 90,
      render: (enabled, record) => (
        <Popconfirm
          title={enabled ? t('确定禁用此规则？') : t('确定启用此规则？')}
          onConfirm={() => handleToggleEnabled(record)}
        >
          <Tag
            color={enabled ? 'green' : 'grey'}
            shape='circle'
            style={{ cursor: 'pointer' }}
          >
            {enabled ? t('已启用') : t('已禁用')}
          </Tag>
        </Popconfirm>
      ),
    },
    {
      title: t('备注'),
      dataIndex: 'remark',
      width: 140,
      render: (text) => text || '-',
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      width: 140,
      fixed: 'right',
      render: (text, record) => (
        <Space>
          <Button
            size='small'
            type='tertiary'
            onClick={() => openEditForm(record)}
          >
            {t('编辑规则')}
          </Button>
          <Popconfirm
            title={t('确定删除此规则？')}
            onConfirm={() => handleDelete(record.id)}
          >
            <Button size='small' type='danger'>
              {t('删除规则')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <>
      <Modal
        title={
          <Space>
            <Text>{t('重置规则管理')}</Text>
            {channelId ? (
              <Tag size='small' shape='circle' color='white'>
                ID: {channelId}
              </Tag>
            ) : null}
          </Space>
        }
        visible={visible}
        onCancel={onClose}
        width={900}
        footer={null}
      >
        <Spin spinning={loading}>
          <div className='flex justify-between items-center mb-3'>
            <Text type='secondary'>
              {t('共 {{count}} 条规则', { count: rules.length })}
            </Text>
            <Button
              type='primary'
              theme='solid'
              onClick={openAddForm}
              disabled={!channelId}
            >
              {t('添加规则')}
            </Button>
          </div>
          <Table
            columns={columns}
            dataSource={rules}
            rowKey='id'
            pagination={false}
            empty={
              <Empty
                image={<IllustrationNoResult />}
                darkModeImage={<IllustrationNoResultDark />}
                description={t('暂无重置规则')}
              />
            }
          />
        </Spin>
      </Modal>

      <Modal
        title={editingRule ? t('编辑规则') : t('添加规则')}
        visible={formVisible}
        onOk={handleSubmit}
        onCancel={closeForm}
        maskClosable={false}
        centered={true}
        size='medium'
        confirmLoading={submitting}
        className='!rounded-lg'
      >
        <Form
          getFormApi={setFormApi}
          key={editingRule ? `edit-${editingRule.id}` : 'add'}
          labelPosition='top'
          initValues={getFormInitValues()}
        >
          <Form.Slot label={t('规则类型')}>
            <Select
              value={ruleType}
              onChange={setRuleType}
              optionList={ruleTypeOptions}
              disabled={!!editingRule}
              style={{ width: '100%' }}
            />
          </Form.Slot>

          {ruleType === 'daily' && (
            <div className='flex gap-2'>
              <Form.InputNumber
                field='hour'
                label={t('小时')}
                min={0}
                max={23}
                style={{ flex: 1 }}
              />
              <Form.InputNumber
                field='minute'
                label={t('分钟')}
                min={0}
                max={59}
                style={{ flex: 1 }}
              />
            </div>
          )}

          {ruleType === 'weekly' && (
            <>
              <Form.Select
                field='weekday'
                label={t('星期')}
                optionList={weekdayOptions}
                style={{ width: '100%' }}
              />
              <div className='flex gap-2'>
                <Form.InputNumber
                  field='hour'
                  label={t('小时')}
                  min={0}
                  max={23}
                  style={{ flex: 1 }}
                />
                <Form.InputNumber
                  field='minute'
                  label={t('分钟')}
                  min={0}
                  max={59}
                  style={{ flex: 1 }}
                />
              </div>
            </>
          )}

          {ruleType === 'monthly' && (
            <div className='flex gap-2'>
              <Form.InputNumber
                field='day_of_month'
                label={t('几号')}
                min={1}
                max={31}
                style={{ flex: 1 }}
              />
              <Form.InputNumber
                field='hour'
                label={t('小时')}
                min={0}
                max={23}
                style={{ flex: 1 }}
              />
              <Form.InputNumber
                field='minute'
                label={t('分钟')}
                min={0}
                max={59}
                style={{ flex: 1 }}
              />
            </div>
          )}

          {ruleType === 'custom_interval' && (
            <Form.InputNumber
              field='interval_seconds'
              label={t('间隔秒数')}
              min={60}
              style={{ width: '100%' }}
            />
          )}

          {ruleType === 'specific_time' && (
            <Form.DatePicker
              field='specific_time'
              label={t('定点时间')}
              type='dateTime'
              style={{ width: '100%' }}
            />
          )}

          <Form.InputNumber
            field='reset_value'
            label={t('重置后配额值')}
            min={-1}
            placeholder={t('0 表示保持不变')}
            style={{ width: '100%' }}
          />

          <Form.Input field='remark' label={t('备注')} placeholder={t('可选')} />
        </Form>
      </Modal>
    </>
  );
};

export default ChannelResetRuleModal;
