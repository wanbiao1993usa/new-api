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
import { Button } from '@douyinfe/semi-ui';
import { ArrowRight, Sparkles } from 'lucide-react';
import { Link } from 'react-router-dom';
import PricingTopSection from '../header/PricingTopSection';
import PricingView from './PricingView';

const PricingContent = ({ isMobile, sidebarProps, ...props }) => {
  return (
    <div
      className={isMobile ? 'pricing-content-mobile' : 'pricing-scroll-hide'}
    >
      {/* 固定的顶部区域（分类介绍 + 搜索和操作） */}
      <div className='pricing-search-header'>
        <div className='mb-2 flex flex-col gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-emerald-900 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-100 md:flex-row md:items-center md:justify-between'>
          <div className='flex min-w-0 items-center gap-2 text-sm font-medium'>
            <Sparkles size={16} className='shrink-0' />
            <span className='truncate'>
              {props.t?.(
                '下方价格已按分组折扣显示；实际扣费以 API Token 所选分组为准',
              )}
            </span>
          </div>
          <Link to='/plans' className='shrink-0'>
            <Button
              size='small'
              type='primary'
              theme='solid'
              icon={<ArrowRight size={14} />}
            >
              {props.t?.('查看计费说明')}
            </Button>
          </Link>
        </div>
        <PricingTopSection
          {...props}
          isMobile={isMobile}
          sidebarProps={sidebarProps}
          showWithRecharge={sidebarProps.showWithRecharge}
          setShowWithRecharge={sidebarProps.setShowWithRecharge}
          currency={sidebarProps.currency}
          setCurrency={sidebarProps.setCurrency}
          showRatio={sidebarProps.showRatio}
          setShowRatio={sidebarProps.setShowRatio}
          viewMode={sidebarProps.viewMode}
          setViewMode={sidebarProps.setViewMode}
          tokenUnit={sidebarProps.tokenUnit}
          setTokenUnit={sidebarProps.setTokenUnit}
        />
      </div>

      {/* 可滚动的内容区域 */}
      <div
        className={
          isMobile ? 'pricing-view-container-mobile' : 'pricing-view-container'
        }
      >
        <PricingView {...props} viewMode={sidebarProps.viewMode} />
      </div>
    </div>
  );
};

export default PricingContent;
