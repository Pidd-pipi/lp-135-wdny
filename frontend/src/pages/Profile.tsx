import { useState, useEffect } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { donationAPI, authAPI } from '../api';
import { Donation, RefundStatus } from '../types';
import RefundModal from '../components/RefundModal';

// 捐赠后 48 小时内可申请退款。
const REFUND_WINDOW_MS = 48 * 60 * 60 * 1000;

const refundStatusStyle: Record<RefundStatus, { label: string; cls: string }> = {
  pending: { label: '退款审核中', cls: 'bg-amber-100 text-amber-700' },
  approved: { label: '已退款', cls: 'bg-gray-200 text-gray-600' },
  rejected: { label: '退款已驳回', cls: 'bg-red-100 text-red-600' },
};

const Profile = () => {
  const { user, logout, updateUser } = useAuth();
  const [donations, setDonations] = useState<Donation[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'info' | 'donations'>('info');
  const [refundTarget, setRefundTarget] = useState<Donation | null>(null);

  useEffect(() => {
    if (user) loadDonations();
  }, [user]);

  const refreshProfile = async () => {
    try {
      const res = await authAPI.getMe();
      if (res.data?.totalDonation !== undefined) {
        updateUser({ totalDonation: res.data.totalDonation, serviceHours: res.data.serviceHours });
      }
    } catch {
      // 静默处理：累计金额刷新失败不影响记录展示
    }
  };

  const loadDonations = async () => {
    try {
      const [response] = await Promise.all([
        donationAPI.getMyDonations({ limit: 20 }),
        refreshProfile(),
      ]);
      setDonations(response.data.donations);
    } catch (error) {
      console.error('加载捐赠记录失败:', error);
    } finally {
      setLoading(false);
    }
  };

  const viewCertificate = async (donationId: string) => {
    try {
      const response = await donationAPI.getCertificate(donationId);
      const cert = response.data.certificate;
      alert(`电子凭证\n\n凭证编号：${cert.certificateNo}\n捐赠金额：¥${cert.amount}\n项目：${cert.projectTitle}\n捐赠人：${cert.donorName}\n捐赠时间：${new Date(cert.createdAt).toLocaleString()}`);
    } catch (error: any) {
      alert(error.response?.data?.message || '获取凭证失败');
    }
  };

  const withinRefundWindow = (d: Donation) =>
    Date.now() - new Date(d.createdAt).getTime() <= REFUND_WINDOW_MS;

  if (!user) {
    return <Navigate to="/login" />;
  }

  const roleMap: Record<string, string> = {
    user: '个人用户',
    org: '公益组织',
    admin: '管理员',
  };

  return (
    <div className="max-w-4xl mx-auto">
      <div className="bg-white rounded-2xl shadow-sm overflow-hidden mb-6">
        <div className="bg-gradient-to-r from-primary-500 to-primary-600 h-32" />
        <div className="px-8 pb-8">
          <div className="flex items-end gap-6 -mt-12 mb-6">
            <div className="w-24 h-24 bg-white rounded-full border-4 border-white shadow-lg flex items-center justify-center">
              <span className="text-3xl font-bold text-primary-600">
                {(user.realName || user.username).charAt(0)}
              </span>
            </div>
            <div className="mb-2">
              <h1 className="text-2xl font-bold text-gray-900">{user.realName || user.username}</h1>
              <p className="text-gray-500">@{user.username}</p>
            </div>
            <span className="mb-3 px-3 py-1 bg-primary-100 text-primary-700 rounded-full text-sm font-medium">
              {roleMap[user.role]}
            </span>
          </div>

          <div className="grid grid-cols-3 gap-8">
            <div className="text-center p-4 bg-gray-50 rounded-xl">
              <div className="text-3xl font-bold text-primary-600">¥{user.totalDonation?.toLocaleString() || 0}</div>
              <div className="text-sm text-gray-500 mt-1">累计捐赠</div>
            </div>
            <div className="text-center p-4 bg-gray-50 rounded-xl">
              <div className="text-3xl font-bold text-green-600">{user.serviceHours || 0}</div>
              <div className="text-sm text-gray-500 mt-1">服务时长</div>
            </div>
            <div className="text-center p-4 bg-gray-50 rounded-xl">
              <div className="text-3xl font-bold text-blue-600">{donations.length}</div>
              <div className="text-sm text-gray-500 mt-1">捐赠次数</div>
            </div>
          </div>
        </div>
      </div>

      <div className="bg-white rounded-2xl shadow-sm">
        <div className="border-b border-gray-100">
          <div className="flex gap-8 px-8">
            <button
              onClick={() => setActiveTab('info')}
              className={`py-4 font-medium ${activeTab === 'info' ? 'text-primary-600 border-b-2 border-primary-600' : 'text-gray-500'}`}
            >
              个人信息
            </button>
            <button
              onClick={() => setActiveTab('donations')}
              className={`py-4 font-medium ${activeTab === 'donations' ? 'text-primary-600 border-b-2 border-primary-600' : 'text-gray-500'}`}
            >
              捐赠记录
            </button>
          </div>
        </div>

        <div className="p-8">
          {activeTab === 'info' && (
            <div className="max-w-lg">
              <div className="space-y-6">
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">用户名</label>
                  <div className="text-gray-900">{user.username}</div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">邮箱</label>
                  <div className="text-gray-900">{user.email}</div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">真实姓名</label>
                  <div className="text-gray-900">{user.realName || '-'}</div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">手机号</label>
                  <div className="text-gray-900">{user.phone || '-'}</div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">注册时间</label>
                  <div className="text-gray-900">{user.createdAt ? new Date(user.createdAt).toLocaleDateString() : '-'}</div>
                </div>
              </div>
              <button
                onClick={logout}
                className="mt-8 px-6 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100"
              >
                退出登录
              </button>
            </div>
          )}

          {activeTab === 'donations' && (
            <div>
              {loading ? (
                <div className="text-center py-8">加载中...</div>
              ) : donations.length === 0 ? (
                <div className="text-center py-12 text-gray-500">
                  暂无捐赠记录
                </div>
              ) : (
                <div className="space-y-4">
                  {donations.map((donation) => {
                    const refunded = donation.paymentStatus === 'refunded';
                    const refund = donation.refund;
                    return (
                    <div key={donation.id} className="flex items-center justify-between p-4 bg-gray-50 rounded-xl">
                      <div className="flex-1">
                        <div className="flex items-center gap-2">
                          <span className={`font-medium ${refunded ? 'text-gray-400 line-through' : 'text-gray-900'}`}>
                            {donation.project?.title}
                          </span>
                          {refunded && <span className="px-2 py-0.5 text-xs bg-gray-200 text-gray-600 rounded">已退款</span>}
                        </div>
                        <div className="text-sm text-gray-500">
                          {new Date(donation.createdAt).toLocaleString()}
                        </div>
                        {donation.certificateNo && (
                          <div className={`text-sm ${refunded ? 'text-gray-400 line-through' : 'text-primary-600'}`}>
                            凭证号：{donation.certificateNo}
                          </div>
                        )}
                        {refund && (
                          <div className="mt-2">
                            <span className={`inline-block px-2 py-0.5 text-xs rounded ${refundStatusStyle[refund.status].cls}`}>
                              {refundStatusStyle[refund.status].label}
                            </span>
                            <span className="ml-2 text-xs text-gray-500">申请时间：{new Date(refund.createdAt).toLocaleString()}</span>
                            {refund.status === 'rejected' && refund.reviewComment && (
                              <div className="text-xs text-red-500 mt-1">驳回原因：{refund.reviewComment}</div>
                            )}
                          </div>
                        )}
                      </div>
                      <div className="text-right">
                        <div className={`text-xl font-bold ${refunded ? 'text-gray-400 line-through' : 'text-primary-600'}`}>
                          ¥{donation.amount.toLocaleString()}
                        </div>
                        <div className="mt-1 space-x-3">
                          {refunded ? (
                            <span className="text-sm text-gray-400">凭证已失效</span>
                          ) : refund ? (
                            <span className="text-sm text-amber-600">退款处理中</span>
                          ) : withinRefundWindow(donation) ? (
                            <>
                              <button
                                onClick={() => viewCertificate(donation.id)}
                                className="text-sm text-primary-600 hover:text-primary-700"
                              >
                                查看凭证
                              </button>
                              <button
                                onClick={() => setRefundTarget(donation)}
                                className="text-sm text-red-500 hover:text-red-600"
                              >
                                申请退款
                              </button>
                            </>
                          ) : (
                            <button
                              onClick={() => viewCertificate(donation.id)}
                              className="text-sm text-primary-600 hover:text-primary-700"
                            >
                              查看凭证
                            </button>
                          )}
                        </div>
                      </div>
                    </div>
                    );
                  })}
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {refundTarget && (
        <RefundModal
          donation={refundTarget}
          isOpen={!!refundTarget}
          onClose={() => setRefundTarget(null)}
          onSuccess={loadDonations}
        />
      )}
    </div>
  );
};

export default Profile;
