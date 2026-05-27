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
import { Image, Modal, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import SkeletonWrapper from '../components/SkeletonWrapper';

const { Text } = Typography;

const Navigation = ({
  mainNavLinks,
  isMobile,
  isLoading,
  userState,
  pricingRequireAuth,
  afterSalesQRCode,
}) => {
  const { t } = useTranslation();
  const [afterSalesVisible, setAfterSalesVisible] = useState(false);

  const renderNavLinks = () => {
    const baseClasses =
      'flex-shrink-0 flex items-center gap-1 font-semibold rounded-md transition-all duration-200 ease-in-out';
    const hoverClasses = 'hover:text-semi-color-primary';
    const spacingClasses = isMobile ? 'p-1' : 'p-2';

    const commonLinkClasses = `${baseClasses} ${spacingClasses} ${hoverClasses}`;

    return mainNavLinks.map((link) => {
      const linkContent = <span>{link.text}</span>;

      if (link.itemKey === 'afterSales') {
        return (
          <button
            key={link.itemKey}
            type='button'
            onClick={() => setAfterSalesVisible(true)}
            className={`${commonLinkClasses} bg-transparent border-0 cursor-pointer text-inherit`}
          >
            {linkContent}
          </button>
        );
      }

      if (link.isExternal) {
        return (
          <a
            key={link.itemKey}
            href={link.externalLink}
            target='_blank'
            rel='noopener noreferrer'
            className={commonLinkClasses}
          >
            {linkContent}
          </a>
        );
      }

      let targetPath = link.to;
      if (link.itemKey === 'console' && !userState.user) {
        targetPath = '/login';
      }
      if (link.itemKey === 'pricing' && pricingRequireAuth && !userState.user) {
        targetPath = '/login';
      }

      return (
        <Link key={link.itemKey} to={targetPath} className={commonLinkClasses}>
          {linkContent}
        </Link>
      );
    });
  };

  return (
    <>
      <nav className='flex flex-1 items-center gap-1 lg:gap-2 mx-2 md:mx-4 overflow-x-auto whitespace-nowrap scrollbar-hide'>
        <SkeletonWrapper
          loading={isLoading}
          type='navigation'
          count={4}
          width={60}
          height={16}
          isMobile={isMobile}
        >
          {renderNavLinks()}
        </SkeletonWrapper>
      </nav>
      <Modal
        title={t('售后')}
        visible={afterSalesVisible}
        footer={null}
        centered
        onCancel={() => setAfterSalesVisible(false)}
      >
        <div className='flex flex-col items-center gap-4 py-2'>
          {afterSalesQRCode ? (
            <>
              <Image
                src={afterSalesQRCode}
                alt={t('微信群二维码')}
                width={240}
                height={240}
                className='max-w-full'
              />
              <Text type='secondary'>{t('微信扫码加入售后群')}</Text>
            </>
          ) : (
            <Text type='secondary'>{t('暂未配置售后微信群二维码')}</Text>
          )}
        </div>
      </Modal>
    </>
  );
};

export default Navigation;
