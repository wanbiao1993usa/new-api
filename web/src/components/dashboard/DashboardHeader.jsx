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
import { Button, DatePicker, Input, Select } from '@douyinfe/semi-ui';
import { RefreshCw, Search } from 'lucide-react';

const DashboardHeader = ({
  getGreeting,
  greetingVisible,
  showSearchModal,
  handleSearchConfirm,
  refresh,
  loading,
  isAdminUser,
  inputs,
  dataExportDefaultTime,
  timeOptions,
  handleInputChange,
  isMobile,
  t,
}) => {
  const ICON_BUTTON_CLASS = 'text-white hover:bg-opacity-80 !rounded-full';
  const ACTION_BUTTON_CLASS =
    'text-white hover:bg-opacity-80 whitespace-nowrap';
  const FILTER_LABEL_CLASS = 'text-xs font-medium text-gray-500';
  const FILTER_FIELD_CLASS = 'flex flex-col gap-1';

  const { start_timestamp, end_timestamp, username } = inputs;

  return (
    <div className='flex flex-col gap-3 mb-4'>
      <div className='flex items-center justify-between gap-3'>
        <h2
          className='min-w-0 truncate text-2xl font-semibold text-gray-800 transition-opacity duration-1000 ease-in-out'
          style={{ opacity: greetingVisible ? 1 : 0 }}
        >
          {getGreeting}
        </h2>
        <div className='flex flex-shrink-0 gap-3 justify-end'>
          {isMobile && (
            <Button
              type='tertiary'
              icon={<Search size={16} />}
              onClick={showSearchModal}
              className={`bg-green-500 hover:bg-green-600 !rounded-lg ${ACTION_BUTTON_CLASS}`}
            >
              {t('搜索条件')}
            </Button>
          )}
          {isMobile && (
            <Button
              type='tertiary'
              icon={<RefreshCw size={16} />}
              onClick={refresh}
              loading={loading}
              className={`bg-blue-500 hover:bg-blue-600 ${ICON_BUTTON_CLASS}`}
            />
          )}
        </div>
      </div>

      {!isMobile && (
        <div className='flex flex-wrap items-end gap-3'>
          <div className={FILTER_FIELD_CLASS}>
            <span className={FILTER_LABEL_CLASS}>{t('起始时间')}</span>
            <DatePicker
              value={start_timestamp}
              type='dateTime'
              onChange={(value) => handleInputChange(value, 'start_timestamp')}
              style={{ width: 220 }}
            />
          </div>

          <div className={FILTER_FIELD_CLASS}>
            <span className={FILTER_LABEL_CLASS}>{t('结束时间')}</span>
            <DatePicker
              value={end_timestamp}
              type='dateTime'
              onChange={(value) => handleInputChange(value, 'end_timestamp')}
              style={{ width: 220 }}
            />
          </div>

          <div className={FILTER_FIELD_CLASS}>
            <span className={FILTER_LABEL_CLASS}>{t('时间粒度')}</span>
            <Select
              value={dataExportDefaultTime}
              optionList={timeOptions}
              onChange={(value) =>
                handleInputChange(value, 'data_export_default_time')
              }
              style={{ width: 120 }}
            />
          </div>

          {isAdminUser && (
            <div className={FILTER_FIELD_CLASS}>
              <span className={FILTER_LABEL_CLASS}>{t('用户名称')}</span>
              <Input
                value={username}
                placeholder={t('可选值')}
                onChange={(value) => handleInputChange(value, 'username')}
                style={{ width: 160 }}
              />
            </div>
          )}

          <Button
            type='tertiary'
            icon={<Search size={16} />}
            onClick={handleSearchConfirm}
            loading={loading}
            className={`bg-green-500 hover:bg-green-600 !rounded-lg ${ACTION_BUTTON_CLASS}`}
          >
            {t('查询')}
          </Button>
        </div>
      )}
    </div>
  );
};

export default DashboardHeader;
