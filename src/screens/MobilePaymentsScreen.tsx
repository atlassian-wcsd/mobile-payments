import React from 'react';
import styled from 'styled-components';
import AppleWalletWidget from '../components/AppleWalletWidget';

const ScreenContainer = styled.div`
  padding: 20px;
  max-width: 800px;
  margin: 0 auto;
`;

const ScreenTitle = styled.h1`
  font-size: 24px;
  font-weight: 600;
  margin-bottom: 24px;
`;

const MobilePaymentsScreen: React.FC = () => {
  return (
    <ScreenContainer>
      <ScreenTitle>Mobile Payments</ScreenTitle>
      <AppleWalletWidget />
    </ScreenContainer>
  );
};

export default MobilePaymentsScreen;