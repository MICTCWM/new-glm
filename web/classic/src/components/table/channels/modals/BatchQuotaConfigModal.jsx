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

import React, { useState } from 'react';
import {
  Modal,
  Form,
  InputNumber,
  Typography,
  CheckboxGroup,
} from '@douyinfe/semi-ui';

// 每日重置时刻选项（0-23 点）
const HOURS_OPTIONS = Array.from({ length: 24 }, (_, h) => ({
  label: `${String(h).padStart(2, '0')}:00`,
  value: h,
}));

const BatchQuotaConfigModal = ({
  showBatchQuotaConfig,
  setShowBatchQuotaConfig,
  batchSetQuotaConfig,
  selectedChannels,
  t,
}) => {
  const [resetHours, setResetHours] = useState([]);
  const [resetMinute, setResetMinute] = useState(0);
  const [submitting, setSubmitting] = useState(false);
  const [formApi, setFormApi] = useState(null);

  const handleOk = async () => {
    if (selectedChannels.length === 0) {
      return;
    }
    const values = formApi ? formApi.getValues() : {};
    const maxCallCount =
      values.max_call_count === undefined || values.max_call_count === null
        ? 0
        : values.max_call_count;
    const minute =
      resetMinute === undefined || resetMinute === null ? 0 : resetMinute;
    setSubmitting(true);
    try {
      await batchSetQuotaConfig({
        max_call_count: maxCallCount,
        reset_hours: resetHours,
        reset_minute: minute,
      });
    } finally {
      setSubmitting(false);
    }
  };

  const handleCancel = () => {
    setShowBatchQuotaConfig(false);
  };

  const handleAfterClose = () => {
    setResetHours([]);
    setResetMinute(0);
    if (formApi) {
      formApi.reset();
    }
  };

  return (
    <Modal
      title={t('批量编辑配额')}
      visible={showBatchQuotaConfig}
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
          max_call_count: 0,
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

        <Form.InputNumber
          field='max_call_count'
          label={t('总配额')}
          placeholder={t('0 表示不限')}
          min={0}
          style={{ width: '100%' }}
          extraText={t(
            '渠道累计成功调用次数达到此值后将不再分配请求',
          )}
        />

        <div className='mt-3'>
          <Typography.Text className='text-sm font-medium text-gray-700 mb-2 block'>
            {t('重置时间（每小时）')}
          </Typography.Text>
          <Typography.Text
            type='tertiary'
            size='small'
            className='mb-2 block'
          >
            {t('选择每天哪些时刻自动清零已用配额（按服务器时区）')}
          </Typography.Text>
          <CheckboxGroup
            options={HOURS_OPTIONS}
            value={resetHours}
            onChange={setResetHours}
          />
        </div>

        <div className='mt-3'>
          <Typography.Text className='text-sm font-medium text-gray-700 mb-2 block'>
            {t('重置分钟')}
          </Typography.Text>
          <InputNumber
            value={resetMinute}
            onChange={(value) =>
              setResetMinute(
                value === undefined || value === null ? 0 : value,
              )
            }
            min={0}
            max={59}
            style={{ width: '100%' }}
          />
        </div>
      </Form>
    </Modal>
  );
};

export default BatchQuotaConfigModal;
