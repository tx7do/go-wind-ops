import {
  useMutation,
  useQuery,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type authenticationservicev1_GenerateCaptchaResponse,
  type authenticationservicev1_LoginRequest,
  type authenticationservicev1_LoginResponse,
  type authenticationservicev1_GetAccessTokensResponse,
  type authenticationservicev1_MfaBackupCodesResponse,
  type authenticationservicev1_MfaSetupStartResponse,
} from '@/api/generated/admin/service/v1';
import { apiClient } from '@/api/client';
import { queryClient } from '@/core';

// ------------------------------
// 登录（Mutation）
// ------------------------------
export function useLogin(
  options?: UseMutationOptions<
    authenticationservicev1_LoginResponse,
    Error,
    authenticationservicev1_LoginRequest
  >,
) {
  return useMutation({
    mutationFn: (req) => apiClient.authenticationService.Login(req),
    ...options,
  });
}

// ------------------------------
// 登录（Mutation - GET）
// ------------------------------
export const loginMutation = queryClient.getMutationCache().build(queryClient, {
  mutationKey: ['login'],
  mutationFn: apiClient.authenticationService.Login,
  retry: 0,
});

// ------------------------------
// MFA 挑战验证（Mutation）
// 提交 operation_id + TOTP 码，后端校验通过后返回 LoginResponse（含真 access_token）。
// 后端暂无独立 MFA 服务（与基座的契约差异）：无 mfaService 时挂拒绝兜底，
// 避免模块求值即崩溃（2026-09-16 Phase 0 冒烟发现）。
// ------------------------------
export const verifyMfaMutation = queryClient.getMutationCache().build(queryClient, {
  mutationKey: ['mfa-verify'],
  mutationFn: (input: { operationId: string; totpCode: string }) => {
    // 后端暂无独立 MFA 服务（与基座的契约差异）：无 mfaService 时挂拒绝兜底，
    // 避免模块求值即崩溃（2026-09-16 Phase 0 冒烟发现）。
    const mfaClient = (
      apiClient as {
        mfaService?: {
          VerifyMFAChallenge: (req: {
            operationId: string;
            totpCode: string;
          }) => Promise<unknown>;
        };
      }
    ).mfaService;
    if (!mfaClient) {
      return Promise.reject(
        new Error('MFA challenge is not available in this backend'),
      );
    }
    return mfaClient.VerifyMFAChallenge(input);
  },
  retry: 0,
});

// ------------------------------
// MFA(TOTP) 绑定管理（Mutation）
// ------------------------------
export function useMfaSetupStart(
  options?: UseMutationOptions<
    authenticationservicev1_MfaSetupStartResponse,
    Error,
    Record<never, never>
  >,
) {
  return useMutation({
    mutationFn: () => apiClient.mfaService.MfaSetupStart({}),
    ...options,
  });
}

export function useMfaSetupConfirm(
  options?: UseMutationOptions<Record<never, never>, Error, { code: string }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.mfaService.MfaSetupConfirm(req),
    ...options,
  });
}

export function useMfaDisable(
  options?: UseMutationOptions<Record<never, never>, Error, { code: string }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.mfaService.MfaDisable(req),
    ...options,
  });
}

export function useMfaBackupCodesRegenerate(
  options?: UseMutationOptions<
    authenticationservicev1_MfaBackupCodesResponse,
    Error,
    void
  >,
) {
  return useMutation({
    mutationFn: () => apiClient.mfaService.MfaBackupCodesRegenerate({}),
    ...options,
  });
}

// ------------------------------
// 我的会话管理（Query/Mutation）
// ------------------------------
export function useGetAccessTokens(
  options?: Partial<UseQueryOptions<authenticationservicev1_GetAccessTokensResponse, Error, authenticationservicev1_GetAccessTokensResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['myAccessTokens'],
    queryFn: () => apiClient.authenticationService.GetAccessTokens({ clientType: 'admin', userId: undefined }),
    ...options,
  });
}

export function useRevokeTokenById(
  options?: UseMutationOptions<Record<never, never>, Error, { jti: string; reason?: string }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.authenticationService.RevokeTokenById(req as never),
    ...options,
  });
}

// ------------------------------
// 登出（Mutation）
// ------------------------------
export function useLogout(options?: UseMutationOptions<{}, Error, {}>) {
  return useMutation({
    mutationFn: () => apiClient.authenticationService.Logout({}),
    ...options,
  });
}

// ------------------------------
// 登出（Mutation - GET）
// ------------------------------
export const logoutMutation = queryClient.getMutationCache().build(queryClient, {
  mutationKey: ['logout'],
  mutationFn: () => apiClient.authenticationService.Logout({}),
  retry: 0,
});

// ------------------------------
// ------------------------------
// 刷新 Token（Mutation）
// ------------------------------
export function useRefreshToken(
  options?: UseMutationOptions<
    authenticationservicev1_LoginResponse,
    Error,
    authenticationservicev1_LoginRequest
  >,
) {
  return useMutation({
    mutationFn: () =>
      apiClient.authenticationService.RefreshToken({
        grant_type: 'refresh_token',
      }),
    ...options,
  });
}

// ------------------------------
// 刷新 Token（Mutation - GET）
// ------------------------------
export const refreshTokenMutation = queryClient.getMutationCache().build(queryClient, {
  mutationKey: ['refreshToken'],
  mutationFn: (binding?: { userId?: number; jti?: string }) =>
    apiClient.authenticationService.RefreshToken({
      grant_type: 'refresh_token',
      user_id: binding?.userId,
      jti: binding?.jti,
    }),
  retry: 0,
});

// ------------------------------
// 获取验证码（Query - GET）
// ------------------------------
export function useGenerateCaptcha(
  options?: UseQueryOptions<authenticationservicev1_GenerateCaptchaResponse, Error>,
) {
  return useQuery({
    queryKey: ['captcha'],
    queryFn: () => apiClient.authenticationService.GenerateCaptcha({}),
    ...options,
  });
}

// ==============================================
// 获取验证码 【给 Store / 外部调用】不带 Hook 的方法
// ==============================================
export async function fetchGenerateCaptcha() {
  return queryClient.fetchQuery({
    queryKey: ['generateCaptcha'],
    queryFn: () => apiClient.authenticationService.GenerateCaptcha({}),
    staleTime: 0,
    retry: 0,
  });
}
