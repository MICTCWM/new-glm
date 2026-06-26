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

import React, { useState, useMemo } from 'react';
import {
  Modal,
  Form,
  Select,
  InputNumber,
  Input,
  Typography,
} from '@douyinfe/semi-ui';

const RULE_TYPES = [
  'daily',
  'weekly',
  'monthly',
  'custom_interval',
  'specific_time',
];

const WEEKDAYS = [0, 1, 2, 3, 4, 5, 6];

// 将 specific_time 字段统一转为 unix 秒级时间戳
const toUnixSeconds = (value) => {
  if (value == null) return 0;
  if (value instanceof Date) return Math.floor(value.getTime() / 1000);
  if (typeof value === 'number') return Math.floor(value);
  if (typeof value === 'string') {
    const parsed = Date.parse(value);
    if (!isNaN(parsed)) return Math.floor(parsed / 1000);
  }
  return 0;
};

// 根据 rule_type 与表单值构造 rule_config JSON 字符串
export const buildRuleConfig = (ruleType, values) => {
  let config = {};
  switch (ruleType) {
    case 'daily':
      config = { hour: values.hour ?? 0, minute: values.minute ?? 0 };
      break;
    case 'weekly':
      config = {
        weekday: values.weekday ?? 0,
        hour: values.hour ?? 0,
        minute: values.minute ?? 0,
      };
      break;
    case 'monthly':
      config = {
        day_of_month: values.day_of_month ?? 1,
        hour: values.hour ?? 0,
        minute: values.minute ?? 0,
      };
      break;
    case 'custom_interval':
      config = { interval_seconds: values.interval_seconds ?? 3600 };
      break;
    case 'specific_time':
      config = { specific_time: toUnixSeconds(values.specific_time) };
      break;
    default:
      console.error('Unknown rule_type:', ruleType);
      return JSON.stringify({});
  }
  return JSON.stringify(config);
};

// 将 rule_config JSON 字符串解析回表单可用的值（specific_time 转为 Date）
export const parseRuleConfig = (ruleType, ruleConfigStr) => {
  if (!ruleConfigStr) return {};
  try {
    const parsed =
      typeof ruleConfigStr === 'string'
        ? JSON.parse(ruleConfigStr)
        : ruleConfigStr;
    if (ruleType === 'specific_time' && parsed.specific_time) {
      return { ...parsed, specific_time: new Date(parsed.specific_time * 1000) };
    }
    return parsed;
  } catch (e) {
    return {};
  }
};

const BatchResetRuleModal = ({
  showBatchResetRule,
  setShowBatchResetRule,
  batchSetChannelResetRule,
  selectedChannels,
  t,
}) => {
  const [ruleType, setRuleType] = useState('daily');
  const [submitting, setSubmitting] = useState(false);
  const [formApi, setFormApi] = useState(null);

  const handleOk = async () => {
    if (selectedChannels.length === 0) {
      return;
    }
    const values = formApi ? formApi.getValues() : {};
    const ruleConfig = buildRuleConfig(ruleType, values);
    const resetValue =
      values.reset_value === undefined || values.reset_value === null
        ? 0
        : values.reset_value;
    const remark = values.remark || '';
    setSubmitting(true);
    try {
      const success = await batchSetChannelResetRule({
        rule_type: ruleType,
        rule_config: ruleConfig,
        reset_value: resetValue,
        enabled: true,
        remark,
      });
      // 成功时 batchSetChannelResetRule 内部已关闭弹窗，失败时保持打开让用户修正
      if (!success) {
        setSubmitting(false);
      }
    } catch {
      setSubmitting(false);
    }
  };

  const handleCancel = () => {
    setShowBatchResetRule(false);
  };

  const handleAfterClose = () => {
    setRuleType('daily');
    if (formApi) {
      formApi.reset();
    }
  };

  const ruleTypeOptions = useMemo(
    () =>
      RULE_TYPES.map((type) => ({
        value: type,
        label: t(
          type === 'daily'
            ? '每天'
            : type === 'weekly'
              ? '每周'
              : type === 'monthly'
                ? '每月'
                : type === 'custom_interval'
                  ? '自定义间隔'
                  : '定点时间',
        ),
      })),
    [t],
  );

  const weekdayOptions = useMemo(
    () =>
      WEEKDAYS.map((d) => ({
        value: d,
        label: t(
          d === 0
            ? '周日'
            : d === 1
              ? '周一'
              : d === 2
                ? '周二'
                : d === 3
                  ? '周三'
                  : d === 4
                    ? '周四'
                    : d === 5
                      ? '周五'
                      : '周六',
        ),
      })),
    [t],
  );

  return (
    <Modal
      title={t('批量设置重置规则')}
      visible={showBatchResetRule}
      onOk={handleOk}
      onCancel={handleCancel}
      afterClose={handleAfterClose}
      maskClosable={false}
      centered={true}
      size='medium'
      confirmLoading={submitting}
      className='!rounded-lg'
    >
      <Form
        getFormApi={setFormApi}
        labelPosition='top'
        initValues={{
          rule_type: 'daily',
          hour: 3,
          minute: 0,
          weekday: 1,
          day_of_month: 1,
          interval_seconds: 3600,
          reset_value: 0,
          remark: '',
        }}
      >
        <div className='mb-4'>
          <Typography.Text type='secondary'>
            {t('已选择 ${count} 个渠道').replace(
              '${count}',
              selectedChannels.length,
            )}
          </Typography.Text>
        </div>

        <Form.Slot label={t('规则类型')}>
          <Select
            value={ruleType}
            onChange={setRuleType}
            optionList={ruleTypeOptions}
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

        <Form.Input
          field='remark'
          label={t('备注')}
          placeholder={t('可选')}
        />
      </Form>
    </Modal>
  );
};

export default BatchResetRuleModal;
